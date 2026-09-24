"""Live person + gender detection for the demo's camera feeds.

Subscribes to each camera's video relay on the Go backend (the same
WebSocket a browser would use to watch a feed), samples one frame every
PROCESS_INTERVAL_S, runs the real YOLO26m person detector and the real
clip_genero CLIP model (same weights and thresholds as
Modelo/LAP01_Multitracking.ipynb, config_lap01.json's "gender" section),
and publishes the results back through the backend's detections relay for
the browser to draw as an overlay.

This is a live-demo simplification of the notebook's pipeline: identity
here is a simple per-camera IoU tracker across the sampled frames, not the
notebook's full appearance+Hungarian multi-camera re-identification.
"""

import asyncio
import json
import os
import time

import cv2
import httpx
import numpy as np
import torch
import websockets
from transformers import CLIPModel, CLIPProcessor
from ultralytics import YOLO

MODEL_DIR = "/Modelo/Modelo"
BACKEND_WS = os.environ.get("BACKEND_WS", "ws://backend:8080")
BACKEND_HTTP = os.environ.get("BACKEND_HTTP", "http://backend:8080")
PROCESS_INTERVAL_S = float(os.environ.get("PROCESS_INTERVAL_S", "1.5"))
DISCOVER_INTERVAL_S = 10.0

# Same thresholds as config_lap01.json's "detector"/"gender" sections.
DETECTOR_CONF = 0.3
DETECTOR_IOU = 0.5
DETECTOR_IMGSZ = 640
GENDER_MIN_DETECTION_CONF = 0.65
GENDER_MIN_HEIGHT = 96
GENDER_MIN_CONFIDENCE = 0.72
GENDER_MIN_MARGIN = 0.18
GENDER_LABELS = ("Hombre", "Mujer")
GENDER_PROMPTS = ("a photo of a man", "a photo of a woman")
SIN_DETERMINAR = "Sin determinar"
IOU_MATCH_THRESHOLD = 0.3

print("Cargando YOLO26m...", flush=True)
yolo = YOLO(os.path.join(MODEL_DIR, "yolo26m.pt"))
PERSON_CLASS_ID = next(k for k, v in yolo.names.items() if v == "person")

print("Cargando clip_genero...", flush=True)
DEVICE = "cuda" if torch.cuda.is_available() else "cpu"
_clip_dir = os.path.join(MODEL_DIR, "clip_genero")
clip_processor = CLIPProcessor.from_pretrained(_clip_dir, local_files_only=True)
clip_model = CLIPModel.from_pretrained(_clip_dir, local_files_only=True).eval().to(DEVICE)
with torch.inference_mode():
    _text_inputs = clip_processor(text=list(GENDER_PROMPTS), return_tensors="pt", padding=True).to(DEVICE)
    # Go through text_model + text_projection directly rather than the
    # get_text_features() shortcut: its return type changed across
    # transformers versions (some return the pooled tensor, some the
    # full BaseModelOutputWithPooling), while this path is stable.
    _text_outputs = clip_model.text_model(**_text_inputs)
    _text_features = clip_model.text_projection(_text_outputs.pooler_output)
    TEXT_FEATURES = _text_features / _text_features.norm(dim=-1, keepdim=True)

print(f"Modelos listos (device={DEVICE}).", flush=True)


def iou(a, b):
    ax1, ay1, ax2, ay2 = a
    bx1, by1, bx2, by2 = b
    ix1, iy1 = max(ax1, bx1), max(ay1, by1)
    ix2, iy2 = min(ax2, bx2), min(ay2, by2)
    iw, ih = max(0.0, ix2 - ix1), max(0.0, iy2 - iy1)
    inter = iw * ih
    if inter <= 0:
        return 0.0
    area_a = max(0.0, ax2 - ax1) * max(0.0, ay2 - ay1)
    area_b = max(0.0, bx2 - bx1) * max(0.0, by2 - by1)
    denom = area_a + area_b - inter
    return inter / denom if denom > 0 else 0.0


class CameraTracker:
    """Assigns stable local ids across sampled frames via greedy IoU
    matching. A lightweight stand-in for the notebook's full tracker,
    sized for a live overlay rather than offline re-identification."""

    def __init__(self):
        self.next_id = 1
        self.tracks = []  # [{"id": int, "box": (x1,y1,x2,y2)}, ...]

    def update(self, boxes):
        used = set()
        ids = []
        for box in boxes:
            best_iou, best_idx = 0.0, -1
            for idx, t in enumerate(self.tracks):
                if idx in used:
                    continue
                v = iou(box, t["box"])
                if v > best_iou:
                    best_iou, best_idx = v, idx
            if best_idx >= 0 and best_iou >= IOU_MATCH_THRESHOLD:
                used.add(best_idx)
                ids.append(self.tracks[best_idx]["id"])
            else:
                ids.append(self.next_id)
                self.next_id += 1
        self.tracks = [{"id": tid, "box": box} for tid, box in zip(ids, boxes)]
        return ids


def classify_gender(crop_rgb):
    with torch.inference_mode():
        inputs = clip_processor(images=crop_rgb, return_tensors="pt").to(DEVICE)
        vision_outputs = clip_model.vision_model(pixel_values=inputs["pixel_values"])
        image_features = clip_model.visual_projection(vision_outputs.pooler_output)
        image_features = image_features / image_features.norm(dim=-1, keepdim=True)
        logit_scale = clip_model.logit_scale.exp()
        logits = (image_features @ TEXT_FEATURES.T) * logit_scale
        probs = logits.softmax(dim=-1)[0].cpu().numpy()
    top = int(probs.argmax())
    conf = float(probs[top])
    margin = float(conf - probs[1 - top])
    if conf < GENDER_MIN_CONFIDENCE or margin < GENDER_MIN_MARGIN:
        return SIN_DETERMINAR, conf
    return GENDER_LABELS[top], conf


def process_frame(jpeg_bytes, tracker):
    arr = np.frombuffer(jpeg_bytes, dtype=np.uint8)
    img = cv2.imdecode(arr, cv2.IMREAD_COLOR)
    if img is None:
        return None
    h, w = img.shape[:2]
    results = yolo.predict(
        img,
        classes=[PERSON_CLASS_ID],
        conf=DETECTOR_CONF,
        iou=DETECTOR_IOU,
        imgsz=DETECTOR_IMGSZ,
        verbose=False,
    )
    boxes, confs = [], []
    for r in results:
        for b in r.boxes:
            x1, y1, x2, y2 = (float(v) for v in b.xyxy[0].tolist())
            boxes.append((x1, y1, x2, y2))
            confs.append(float(b.conf[0]))
    ids = tracker.update(boxes)
    people = []
    for (x1, y1, x2, y2), conf, pid in zip(boxes, confs, ids):
        entry = {
            "id": pid,
            "box": [round(x1, 1), round(y1, 1), round(x2, 1), round(y2, 1)],
            "conf": round(conf, 3),
            "gender": None,
            "gender_conf": None,
        }
        if conf >= GENDER_MIN_DETECTION_CONF and (y2 - y1) >= GENDER_MIN_HEIGHT:
            crop = img[max(0, int(y1)) : int(y2), max(0, int(x1)) : int(x2)]
            if crop.size > 0:
                crop_rgb = cv2.cvtColor(crop, cv2.COLOR_BGR2RGB)
                gender, gconf = classify_gender(crop_rgb)
                entry["gender"] = gender
                entry["gender_conf"] = round(gconf, 3)
        people.append(entry)
    return {"ts": time.time(), "frame_w": w, "frame_h": h, "people": people}


async def discover_cameras():
    try:
        async with httpx.AsyncClient(timeout=5) as client:
            resp = await client.get(f"{BACKEND_HTTP}/api/v1/map-objects")
            resp.raise_for_status()
            objs = resp.json()
            return [o["id"] for o in objs if o.get("kind") == "camera"]
    except Exception as exc:  # backend not ready yet, or transient network error
        print("discover_cameras:", exc, flush=True)
        return []


async def run_camera(camera_id):
    tracker = CameraTracker()
    watch_url = f"{BACKEND_WS}/api/v1/cameras/{camera_id}/watch"
    publish_url = f"{BACKEND_WS}/api/v1/cameras/{camera_id}/detections/publish"
    while True:
        try:
            async with websockets.connect(watch_url, max_size=2**20) as watch_ws:
                async with websockets.connect(publish_url, max_size=2**16) as publish_ws:
                    print(f"[{camera_id}] conectado", flush=True)
                    last_processed = 0.0
                    async for message in watch_ws:
                        if not isinstance(message, (bytes, bytearray)):
                            continue
                        now = time.monotonic()
                        if now - last_processed < PROCESS_INTERVAL_S:
                            continue
                        last_processed = now
                        result = await asyncio.to_thread(process_frame, message, tracker)
                        if result is not None:
                            await publish_ws.send(json.dumps(result))
        except Exception as exc:
            print(f"[{camera_id}] error, reintentando en 3s: {exc}", flush=True)
            await asyncio.sleep(3)


async def main():
    running = {}
    while True:
        for camera_id in await discover_cameras():
            task = running.get(camera_id)
            if task is None or task.done():
                print("Iniciando cámara", camera_id, flush=True)
                running[camera_id] = asyncio.create_task(run_camera(camera_id))
        await asyncio.sleep(DISCOVER_INTERVAL_S)


if __name__ == "__main__":
    asyncio.run(main())
