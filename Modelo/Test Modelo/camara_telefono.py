"""Cámara del teléfono en vivo: lee el MJPEG de la app Android, corre el modelo final (GPU) y publica en la web."""
import argparse
import json
import os
import sys
import threading
import time
import urllib.request
import uuid
import warnings
from datetime import datetime
from pathlib import Path
from urllib.parse import urlsplit

TEST = Path(__file__).resolve().parent
MODELO = TEST / "Modelo"
(MODELO / ".runtime" / "ultralytics").mkdir(parents=True, exist_ok=True)
os.environ["YOLO_CONFIG_DIR"] = str(MODELO / ".runtime" / "ultralytics")
os.environ["YOLO_OFFLINE"] = "1"
os.environ["YOLO_AUTOINSTALL"] = "0"
sys.path.insert(0, str(MODELO))

import cv2
import numpy as np
import torch
from websockets.sync.client import connect

warnings.filterwarnings("ignore", category=DeprecationWarning, module=r"websockets(\..*)?|__main__")

import lap01

CAMARA = "esan-movil"
ANCHO_RELEVO = 640


def leer_json(url, timeout=5):
    """GET de un JSON de la web."""
    with urllib.request.urlopen(url, timeout=timeout) as respuesta:
        return json.loads(respuesta.read().decode("utf-8"))


def fuente_telefono(url_web):
    """URL del MJPEG del teléfono guardada en la web (Cámara del teléfono), o None."""
    camaras = leer_json(f"{url_web}/api/v1/esan/config")["cameras"]
    return next((c["stream_uri"] or None for c in camaras if c["camera_id"] == CAMARA), None)


def sin_token(url):
    """La URL sin su query (para mostrarla sin exponer el token)."""
    partes = urlsplit(url)
    return f"{partes.scheme}://{partes.netloc}{partes.path}"


class LectorMjpeg:
    """Lee el MJPEG del teléfono en un hilo y guarda solo el último frame."""

    def __init__(self, url):
        self.url, self.cap = url, cv2.VideoCapture(url, cv2.CAP_FFMPEG)
        fps = self.cap.get(cv2.CAP_PROP_FPS)
        self.fps = fps if 1 <= fps <= 60 else 15.0
        self._ultimo, self._lock, self._parar = None, threading.Lock(), threading.Event()
        self.leidos, self.terminado = 0, False
        self._hilo = threading.Thread(target=self._leer, daemon=True)
        self._hilo.start()

    def _leer(self):
        """Lee frames mientras lleguen; varios fallos seguidos cierran la fuente."""
        fallos = 0
        while not self._parar.is_set() and fallos < 30:
            ok, frame = self.cap.read() if self.cap.isOpened() else (False, None)
            if not ok:
                fallos += 1
                time.sleep(0.1)
                continue
            fallos = 0
            self.leidos += 1
            with self._lock:
                self._ultimo = (self.leidos, frame, time.perf_counter())
        self.terminado = True
        self.cap.release()

    def tomar(self, despues_de):
        """(número, frame, llegada) del frame más reciente posterior a `despues_de`, o None."""
        with self._lock:
            return self._ultimo if self._ultimo is not None and self._ultimo[0] > despues_de else None

    def detener(self):
        """Termina la lectura."""
        self._parar.set()
        self._hilo.join(timeout=3)


class Relevo:
    """Conexiones WebSocket con la web: video del teléfono y detecciones para dibujar encima."""

    def __init__(self, url_web):
        base = url_web.replace("https://", "wss://").replace("http://", "ws://")
        self.urls = {"video": f"{base}/api/v1/cameras/{CAMARA}/publish", "detecciones": f"{base}/api/v1/cameras/{CAMARA}/detections/publish"}
        self.sockets, self.reintento = {}, {}

    def _enviar(self, canal, mensaje):
        """Envía por un canal, reconectando si hace falta (sin frenar el lazo si la web no está)."""
        if time.monotonic() < self.reintento.get(canal, 0):
            return
        try:
            if canal not in self.sockets:
                self.sockets[canal] = connect(self.urls[canal], max_size=2 ** 20, open_timeout=3)
            self.sockets[canal].send(mensaje)
        except Exception:
            socket = self.sockets.pop(canal, None)
            if socket is not None:
                socket.close()
            self.reintento[canal] = time.monotonic() + 2

    def video(self, frame):
        """Publica el frame (reducido a 640 px) para que la web lo muestre."""
        alto = round(frame.shape[0] * ANCHO_RELEVO / frame.shape[1])
        ok, jpeg = cv2.imencode(".jpg", cv2.resize(frame, (ANCHO_RELEVO, alto), interpolation=cv2.INTER_AREA),
                                [cv2.IMWRITE_JPEG_QUALITY, 70])
        if ok:
            self._enviar("video", jpeg.tobytes())

    def detecciones(self, mensaje):
        """Publica las cajas, IDs, género y estado del modelo."""
        self._enviar("detecciones", json.dumps(mensaje))

    def estado(self, estado, texto):
        """Publica un estado sin detecciones (esperando fuente, sin conexión...)."""
        self.detecciones({"ts": time.time(), "frame_w": 0, "frame_h": 0, "people": [], "estado": {"estado": estado, "mensaje": texto}})

    def cerrar(self):
        """Cierra las conexiones."""
        for socket in self.sockets.values():
            socket.close()
        self.sockets = {}


class SesionTelefono:
    """Una sesión en vivo del modelo final sobre la cámara del teléfono, guardada en la base cada `checkpoint_s`."""

    def __init__(self, motor, asociador, config, url_web, relevo, primer_frame, fps, fuente, checkpoint_s):
        self.motor, self.asociador, self.config, self.url_web, self.relevo = motor, asociador, config, url_web, relevo
        self.fuente, self.checkpoint_s, self.fps = fuente, checkpoint_s, fps
        alto, ancho = primer_frame.shape[:2]
        self.metadata = {CAMARA: {"camera_id": CAMARA, "video": sin_token(fuente), "fps": fps, "frames": 0, "duration_s": 0.0,
                                  "width": ancho, "height": alto, "area": (0, 0, ancho, alto)}}
        self.session_uuid = uuid.uuid4()
        self.session_id = datetime.now().strftime("%Y%m%d_%H%M%S") + "_" + self.session_uuid.hex[:6]
        self.inicio = datetime.now().astimezone()
        self.nombre = f"Teléfono · {self.inicio:%Y-%m-%d %H:%M}"
        motor.reiniciar({CAMARA: fps})
        asociador.reiniciar(self.session_uuid)
        asociador.areas = {CAMARA: (0, 0, ancho, alto)}
        motor.calentar({CAMARA: primer_frame})
        self.filas, self.latencias = [], []
        self.vueltas, self.saltados, self.ultimo, self.t = 0, 0, 0, -1.0
        self.t0, self.ultimo_guardado, self.busy_s = time.perf_counter(), time.perf_counter(), 0.0
        self.guardando = None

    def procesar(self, numero, frame, llegada):
        """Detecta, sigue, estima género y asigna global_id al frame; publica el resultado en la web."""
        inicio = time.perf_counter()
        self.saltados += max(0, numero - self.ultimo - 1)
        self.ultimo = numero
        self.vueltas += 1
        self.t = max(self.t + 1e-3, inicio - self.t0)
        filas = lap01.procesar_instante(self.motor, self.asociador, self.t, {CAMARA: frame}, {CAMARA: self.vueltas})[CAMARA]
        latencia = time.perf_counter() - llegada
        self.latencias.append(latencia)
        for fila in filas:
            fila.update(X=None, Y=None, speed=None, direction_deg=None, projection_valid=False)
        self.filas.extend({**fila, "frame_global": self.vueltas, "timestamp_s": self.t, "camera_id": CAMARA, "frame": numero,
                           "global_id_online": fila["global_id"], "latencia_s": latencia} for fila in filas)
        escala = ANCHO_RELEVO / frame.shape[1]
        personas = []
        for fila in filas:
            genero = fila.get("genero") if fila.get("genero") in ("Hombre", "Mujer") else None
            personas.append({"id": fila["global_id"] if fila["global_id"] is not None else fila["local_id"],
                             "global_id": fila["global_id"], "local_id": fila["local_id"],
                             "box": [round(fila[k] * escala, 1) for k in ("x1", "y1", "x2", "y2")],
                             "conf": round(fila["confidence"], 3), "gender": genero,
                             "gender_conf": round(fila["confianza_genero"], 3) if genero and fila.get("confianza_genero") else None})
        multi = self.asociador.resumen()
        reloj = time.perf_counter() - self.t0
        self.relevo.video(frame)
        self.relevo.detecciones({"ts": time.time(), "frame_w": ANCHO_RELEVO, "frame_h": round(frame.shape[0] * escala), "people": personas,
                                 "estado": {"estado": "procesando", "fuente": sin_token(self.fuente), "sesion": str(self.session_uuid),
                                            "fps": round(self.vueltas / max(reloj, 1e-6), 1),
                                            "latencia_ms": round(1000 * float(np.median(self.latencias[-30:])), 0),
                                            "saltados": self.saltados, "personas_ahora": len(personas),
                                            "personas_total": multi["identidades_globales"], "segundos": round(reloj, 1),
                                            "dispositivo": self.motor.device}})
        self.busy_s += time.perf_counter() - inicio
        if time.perf_counter() - self.ultimo_guardado >= self.checkpoint_s:
            self.guardar("RUNNING")

    def guardar(self, status):
        """Reconcilia los IDs y guarda la sesión completa en la base (en segundo plano salvo al cerrar)."""
        self.ultimo_guardado = time.perf_counter()
        if not self.filas or (self.guardando is not None and self.guardando.is_alive() and status == "RUNNING"):
            return
        resultado = lap01.armar_resultado(
            self.motor, self.asociador, self.filas, metadata=self.metadata, inicios={CAMARA: 0}, offsets={CAMARA: 0.0},
            counts={CAMARA: self.vueltas}, fps={CAMARA: self.fps}, status="completo" if status == "DONE" else "en_vivo",
            session_uuid=self.session_uuid, session_id=self.session_id, inicio_grabacion=self.inicio,
            first_timestamp=self.filas[0]["timestamp_s"], last_timestamp=self.t,
            elapsed=time.perf_counter() - self.t0, busy_s=self.busy_s)
        resultado.resumen["tiempo_real"] = {"fuente": sin_token(self.fuente), "saltados": self.saltados,
                                            "latencia_mediana_ms": round(1000 * float(np.median(self.latencias)), 1)}
        carga = lap01.sesion_para_bd(resultado, nombre=self.nombre, kind="LIVE", status=status, config=self.config,
                                     camaras_nombres={CAMARA: "Cámara del teléfono"})
        carga["cameras"] = []
        enviar = lambda: self._publicar(carga, status)
        if status == "RUNNING":
            self.guardando = threading.Thread(target=enviar, daemon=True)
            self.guardando.start()
        else:
            if self.guardando is not None:
                self.guardando.join(timeout=30)
            enviar()

    def _publicar(self, carga, status):
        """POST de la sesión; un fallo solo se informa, no corta la cámara."""
        try:
            r = lap01.publicar_sesion(carga, self.url_web)
            print(f"[{datetime.now():%H:%M:%S}] sesión {status}: {r['identities']} personas, {r['points']} puntos ({r['ms']} ms)", flush=True)
        except Exception as error:
            print(f"No se pudo guardar la sesión: {error}", flush=True)


def correr_fuente(motor, asociador, config, url_web, relevo, fuente, checkpoint_s):
    """Procesa la fuente hasta que se corte, cambie la URL en la web o se pulse Ctrl+C."""
    lector = LectorMjpeg(fuente)
    limite = time.monotonic() + 10
    while lector.tomar(0) is None and not lector.terminado and time.monotonic() < limite:
        time.sleep(0.05)
    dato = lector.tomar(0)
    if dato is None:
        lector.detener()
        relevo.estado("sin_conexion", f"No se pudo leer {sin_token(fuente)}: revisa que la app esté transmitiendo y la misma red.")
        print(f"Sin conexión con {sin_token(fuente)}", flush=True)
        return
    sesion = SesionTelefono(motor, asociador, config, url_web, relevo, dato[1], lector.fps, fuente, checkpoint_s)
    print(f"Procesando {sin_token(fuente)} · sesión {sesion.session_uuid} · {motor.device}", flush=True)
    revisado = time.monotonic()
    try:
        while not lector.terminado:
            dato = lector.tomar(sesion.ultimo)
            if dato is None:
                time.sleep(0.002)
            else:
                sesion.procesar(*dato)
            if time.monotonic() - revisado > 3:
                revisado = time.monotonic()
                try:
                    if fuente_telefono(url_web) != fuente:
                        print("La URL cambió en la web: se cierra esta sesión.", flush=True)
                        break
                except OSError:
                    pass
    finally:
        lector.detener()
        sesion.guardar("DONE")


def main():
    """Espera la URL del teléfono en la web y procesa mientras esté disponible."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--web", default="http://localhost:8080", help="URL de la web (nginx)")
    parser.add_argument("--checkpoint", type=float, default=15.0, help="segundos entre guardados en la base")
    args = parser.parse_args()
    url_web = args.web.rstrip("/")
    config = json.loads((MODELO / "config_lap01.json").read_text())
    camaras = json.loads((MODELO / "camaras.json").read_text())
    config_camara = {"mode": "visual_temporal", "units": "m", "cameras": {CAMARA: {"timestamp_offset": 0.0}},
                     "overlaps": [], "transitions": [], "association": camaras.get("association", {})}
    print("Cargando el modelo final (YOLO26m, tracker, Re-ID, género)...", flush=True)
    motor = lap01.MotorLAP01(MODELO, config, device="auto", batch=True)
    asociador = lap01.crear_asociador(motor, config_camara)
    print(f"Modelo listo en {motor.device} · GPU: {torch.cuda.get_device_name(0) if torch.cuda.is_available() else 'no'}", flush=True)
    relevo = Relevo(url_web)
    try:
        while True:
            try:
                fuente = fuente_telefono(url_web)
            except OSError as error:
                print(f"La web no responde en {url_web} ({error}); reintento en 3 s", flush=True)
                time.sleep(3)
                continue
            if not fuente:
                relevo.estado("sin_fuente", "Pega en la web la URL que muestra la app Cámara ESAN.")
                time.sleep(2)
                continue
            correr_fuente(motor, asociador, config, url_web, relevo, fuente, args.checkpoint)
            time.sleep(2)
    except KeyboardInterrupt:
        print("Detenido.", flush=True)
    finally:
        relevo.estado("detenido", "El servicio del modelo está detenido.")
        relevo.cerrar()


if __name__ == "__main__":
    main()
