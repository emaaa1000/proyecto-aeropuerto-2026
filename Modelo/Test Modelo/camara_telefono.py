"""Teléfonos en vivo: lee el MJPEG de cada app Cámara ESAN, corre el modelo final (GPU) sobre todos a la vez y publica en la web.

No se guarda nada, ni en la base ni en disco: identidades, recorridos y conteos viven en la memoria de este proceso
y se borran al cerrarlo (o al cambiar la lista de teléfonos en la web).
"""
import argparse
import json
import os
import sys
import threading
import time
import urllib.request
import warnings
from collections import Counter, deque
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

CANAL_ESTADO = "telefonos"
ANCHO_RELEVO = 640
FPS_NOMINAL = 15.0


def leer_json(url, timeout=5):
    """GET de un JSON de la web."""
    with urllib.request.urlopen(url, timeout=timeout) as respuesta:
        return json.loads(respuesta.read().decode("utf-8"))


def sin_token(url):
    """La URL sin su query (para mostrarla sin exponer el token)."""
    partes = urlsplit(url)
    return f"{partes.scheme}://{partes.netloc}{partes.path}"


class LectorMjpeg:
    """Lee el MJPEG de un teléfono en un hilo, reconectando si se corta, y guarda solo el último frame."""

    def __init__(self, url):
        self.url = url
        self.leidos, self.error = 0, None
        self._ultimo, self._lock, self._parar = None, threading.Lock(), threading.Event()
        self._hilo = threading.Thread(target=self._leer, daemon=True)
        self._hilo.start()

    @property
    def conectado(self):
        """Hubo un frame en los últimos 3 s."""
        with self._lock:
            return self._ultimo is not None and time.perf_counter() - self._ultimo[2] < 3

    @property
    def estado(self):
        """procesando (llegan frames), sin_conexion (falló el último intento) o conectando."""
        return "procesando" if self.conectado else ("sin_conexion" if self.error else "conectando")

    def _leer(self):
        """Conecta y lee partes multipart (con Content-Length); ante un fallo espera 2 s y reintenta."""
        while not self._parar.is_set():
            try:
                with urllib.request.urlopen(self.url, timeout=5) as respuesta:
                    self._leer_partes(respuesta)
            except Exception as error:
                self.error = f"No se pudo leer {sin_token(self.url)}: {getattr(error, 'reason', None) or error}"
            self._parar.wait(2)

    def _leer_partes(self, respuesta):
        """Decodifica cada JPEG del stream y lo deja como el más reciente."""
        while not self._parar.is_set():
            linea = respuesta.readline(1024)
            if not linea:
                raise ConnectionError("el teléfono cerró la transmisión")
            if not linea.startswith(b"--"):
                continue
            largo = None
            while cabecera := respuesta.readline(1024).strip():
                nombre, _, valor = cabecera.partition(b":")
                if nombre.strip().lower() == b"content-length":
                    largo = int(valor)
            if not largo:
                raise ValueError("el stream MJPEG no indica Content-Length")
            frame = cv2.imdecode(np.frombuffer(respuesta.read(largo), np.uint8), cv2.IMREAD_COLOR)
            if frame is None:
                continue
            self.leidos += 1
            self.error = None
            with self._lock:
                self._ultimo = (self.leidos, frame, time.perf_counter())

    def tomar(self, despues_de):
        """(número, frame, llegada) del frame más reciente posterior a `despues_de`, o None."""
        with self._lock:
            return self._ultimo if self._ultimo is not None and self._ultimo[0] > despues_de else None

    def detener(self):
        """Termina la lectura (el hilo sale en cuanto vence su espera)."""
        self._parar.set()


class Relevo:
    """WebSockets con la web: video y detecciones de cada teléfono, más el estado global del servicio."""

    def __init__(self, url_web):
        self.base = url_web.replace("https://", "wss://").replace("http://", "ws://") + "/api/v1/cameras"
        self.sockets, self.reintento = {}, {}

    def _enviar(self, ruta, mensaje):
        """Envía por un canal, reconectando si hace falta (sin frenar el lazo si la web no está)."""
        if time.monotonic() < self.reintento.get(ruta, 0):
            return
        try:
            if ruta not in self.sockets:
                self.sockets[ruta] = connect(f"{self.base}/{ruta}", max_size=2 ** 20, open_timeout=3)
            self.sockets[ruta].send(mensaje)
        except Exception:
            socket = self.sockets.pop(ruta, None)
            if socket is not None:
                socket.close()
            self.reintento[ruta] = time.monotonic() + 2

    def video(self, cid, frame):
        """Publica el frame (reducido a 640 px) para que la web lo muestre."""
        alto = round(frame.shape[0] * ANCHO_RELEVO / frame.shape[1])
        ok, jpeg = cv2.imencode(".jpg", cv2.resize(frame, (ANCHO_RELEVO, alto), interpolation=cv2.INTER_AREA),
                                [cv2.IMWRITE_JPEG_QUALITY, 70])
        if ok:
            self._enviar(f"{cid}/publish", jpeg.tobytes())

    def detecciones(self, cid, mensaje):
        """Publica las cajas, IDs y género de un teléfono."""
        self._enviar(f"{cid}/detections/publish", json.dumps(mensaje))

    def estado(self, mensaje):
        """Publica el estado global del servicio y de cada teléfono."""
        self._enviar(f"{CANAL_ESTADO}/detections/publish", json.dumps({"ts": time.time(), **mensaje}))

    def cerrar(self):
        """Cierra las conexiones."""
        for socket in self.sockets.values():
            socket.close()
        self.sockets = {}


class SesionEnMemoria:
    """El modelo final sobre un conjunto fijo de teléfonos; Re-ID compartido para que una persona tenga un solo ID en todos."""

    def __init__(self, motor, reid, asociacion, telefonos, clave):
        self.motor, self.telefonos, self.clave = motor, telefonos, clave
        config = {"mode": "visual_temporal", "units": "m", "cameras": {cid: {"timestamp_offset": 0.0} for cid in telefonos},
                  "overlaps": [], "transitions": [], "association": asociacion}
        self.asociador = lap01.AsociadorMulticamara(reid, config)
        motor.reiniciar({cid: FPS_NOMINAL for cid in telefonos})
        self.t0, self.t = time.perf_counter(), -1.0
        self.ultimo = {cid: 0 for cid in telefonos}
        self.procesados = {cid: 0 for cid in telefonos}
        self.saltados = {cid: 0 for cid in telefonos}
        self.latencias = {cid: deque(maxlen=30) for cid in telefonos}
        self.tiempos = {cid: deque(maxlen=30) for cid in telefonos}
        self.personas = {cid: 0 for cid in telefonos}
        self.generos = {}

    def procesar(self, datos):
        """Un instante con el frame nuevo de cada teléfono que lo tenga; devuelve las filas por teléfono."""
        self.t = max(self.t + 1e-3, time.perf_counter() - self.t0)
        frames = {}
        for cid, (numero, frame, _) in datos.items():
            self.saltados[cid] += max(0, numero - self.ultimo[cid] - 1)
            self.ultimo[cid] = numero
            self.procesados[cid] += 1
            frames[cid] = frame
        filas = lap01.procesar_instante(self.motor, self.asociador, self.t, frames, dict(self.procesados))
        ahora = time.perf_counter()
        for cid, (_, _, llegada) in datos.items():
            self.latencias[cid].append(ahora - llegada)
            self.tiempos[cid].append(ahora)
            self.personas[cid] = len(filas[cid])
            for fila in filas[cid]:
                if fila["global_id"] is not None:
                    self.generos[fila["global_id"]] = fila.get("genero") or "Sin determinar"
        return filas

    def fps(self, cid):
        """FPS procesados de un teléfono en sus últimos 30 frames."""
        t = self.tiempos[cid]
        return round((len(t) - 1) / (t[-1] - t[0]), 1) if len(t) > 1 and t[-1] > t[0] else 0.0

    def resumen(self):
        """Personas únicas (entre todos los teléfonos) y su género, contadas solo en memoria."""
        multi = self.asociador.resumen()
        vigentes = set(self.asociador.confirmadas.values())
        return {"segundos": round(time.perf_counter() - self.t0, 1), "personas_total": multi["identidades_globales"],
                "multitelefono": multi["identidades_multicamara"],
                "genero": dict(Counter(g for publico, g in self.generos.items() if publico in vigentes))}


def personas_para_web(filas, ancho):
    """Cajas escaladas al video relevado, con ID global (o local mientras no se confirma) y género."""
    escala = ANCHO_RELEVO / ancho
    personas = []
    for fila in filas:
        genero = fila.get("genero") if fila.get("genero") in ("Hombre", "Mujer") else None
        personas.append({"id": fila["global_id"] if fila["global_id"] is not None else fila["local_id"],
                         "global_id": fila["global_id"], "local_id": fila["local_id"],
                         "box": [round(fila[k] * escala, 1) for k in ("x1", "y1", "x2", "y2")],
                         "conf": round(fila["confidence"], 3), "gender": genero,
                         "gender_conf": round(fila["confianza_genero"], 3) if genero and fila.get("confianza_genero") else None})
    return personas


def main():
    """Sigue la lista de teléfonos de la web y procesa todos los que estén transmitiendo."""
    parser = argparse.ArgumentParser(description=__doc__)
    # 127.0.0.1 y no localhost: en Windows localhost prueba antes ::1 (nginx no escucha ahí) y cada petición tarda ~2 s.
    parser.add_argument("--web", default="http://127.0.0.1:8080", help="URL de la web (nginx)")
    args = parser.parse_args()
    url_web = args.web.rstrip("/")
    config = json.loads((MODELO / "config_lap01.json").read_text())
    asociacion = json.loads((MODELO / "camaras.json").read_text()).get("association", {})
    print("Cargando el modelo final (YOLO26m, tracker, Re-ID, género)...", flush=True)
    motor = lap01.MotorLAP01(MODELO, config, device="auto", batch=True)
    reid = lap01.crear_asociador(motor, {"mode": "visual_temporal", "units": "m", "cameras": {"x": {}}}).reid
    print(f"Modelo listo en {motor.device} · GPU: {torch.cuda.get_device_name(0) if torch.cuda.is_available() else 'no'}", flush=True)
    print("Nada se guarda: todo queda en memoria y se borra al cerrar (Ctrl+C).", flush=True)
    relevo = Relevo(url_web)
    lectores, nombres, sesion, calentado = {}, {}, None, False
    revisado, publicado = 0.0, 0.0
    try:
        while True:
            if time.monotonic() - revisado > 2:
                revisado = time.monotonic()
                try:
                    lista = {t["id"]: t for t in leer_json(f"{url_web}/api/v1/telefonos")}
                except OSError as error:
                    print(f"La web no responde en {url_web} ({error}); reintento en 2 s", flush=True)
                    lista = None
                if lista is not None:
                    for cid in [c for c in lectores if c not in lista or lista[c]["url"] != lectores[c].url]:
                        lectores.pop(cid).detener()
                    for cid, telefono in lista.items():
                        if cid not in lectores:
                            lectores[cid] = LectorMjpeg(telefono["url"])
                    nombres = {cid: t["nombre"] for cid, t in lista.items()}
                    clave = sorted((cid, lector.url) for cid, lector in lectores.items())
                    if (sesion.clave if sesion else []) != clave:
                        sesion = SesionEnMemoria(motor, reid, asociacion, sorted(lectores), clave) if lectores else None
                        print(f"Sesión en memoria con {len(lectores)} teléfono(s): {', '.join(nombres.values()) or 'ninguno'}", flush=True)

            datos = {}
            if sesion is not None:
                for cid in sesion.telefonos:
                    dato = lectores[cid].tomar(sesion.ultimo[cid])
                    if dato is not None:
                        datos[cid] = dato
            if datos:
                if not calentado:
                    motor.calentar({cid: d[1] for cid, d in datos.items()})
                    calentado = True
                filas = sesion.procesar(datos)
                for cid, (_, frame, _) in datos.items():
                    relevo.video(cid, frame)
                    relevo.detecciones(cid, {"ts": time.time(), "frame_w": ANCHO_RELEVO,
                                             "frame_h": round(frame.shape[0] * ANCHO_RELEVO / frame.shape[1]),
                                             "people": personas_para_web(filas[cid], frame.shape[1])})
            else:
                time.sleep(0.003)

            if time.monotonic() - publicado > 0.5:
                publicado = time.monotonic()
                telefonos = {}
                for cid, lector in lectores.items():
                    estado = lector.estado
                    telefonos[cid] = {"nombre": nombres.get(cid, cid), "fuente": sin_token(lector.url), "estado": estado,
                                      "mensaje": lector.error if estado == "sin_conexion" else None}
                    if estado == "procesando" and sesion is not None:
                        telefonos[cid].update(fps=sesion.fps(cid), saltados=sesion.saltados[cid], personas_ahora=sesion.personas[cid],
                                              latencia_ms=round(1000 * float(np.median(sesion.latencias[cid])), 0) if sesion.latencias[cid] else None)
                relevo.estado({"estado": "procesando" if lectores else "sin_telefonos", "dispositivo": motor.device,
                               "telefonos": telefonos, **(sesion.resumen() if sesion is not None else {})})
    except KeyboardInterrupt:
        print("Detenido. Lo procesado se descarta (no se guardó nada).", flush=True)
    finally:
        for lector in lectores.values():
            lector.detener()
        relevo.estado({"estado": "detenido", "telefonos": {}})
        relevo.cerrar()


if __name__ == "__main__":
    main()
