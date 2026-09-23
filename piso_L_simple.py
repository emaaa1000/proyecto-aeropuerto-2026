#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""
Piso en forma de L que ABARQUE TODO
"""

import json
import numpy as np
import cv2
import pandas as pd
from pathlib import Path

BASE = Path(".").resolve()
MODELO_DIR = BASE / "Modelo" / "Modelo"
OUTPUT_DIR = BASE / "Modelo" / "Ouput"

print("=" * 80)
print("PISO L - ABARCAR TODO CORRECTAMENTE")
print("=" * 80)

# Cargar datos
with open(MODELO_DIR / "mapa_piso.json", "r") as f:
    mapa = json.load(f)

trajectory_df = pd.read_csv(OUTPUT_DIR / "trajectory_points.csv")
valid_points = trajectory_df[trajectory_df['x'].notna()]

x_min, x_max = valid_points['x'].min(), valid_points['x'].max()
y_min, y_max = valid_points['y'].min(), valid_points['y'].max()

print(f"\nRango de zonas donde caminaron:")
print(f"  X: [{x_min:.2f}, {x_max:.2f}]")
print(f"  Y: [{y_min:.2f}, {y_max:.2f}]")

# L COMO PASILLOS - ancho consistente, pegado a los puntos, con angulo
# Analizar distribucion de puntos para encontrar los pasillos reales

margen = 0.8  # Margen minimo pegado a los datos

# Ancho de pasillo consistente (ambos pasillos igual)
ancho_pasillo = 9.0

# Pasillo horizontal: corre de izquierda a derecha, centrado en Y de los datos bajos
pasillo_h_y_centro = (y_min + y_max) / 2 - 0.3
pasillo_h_y_min = pasillo_h_y_centro - ancho_pasillo / 2
pasillo_h_y_max = pasillo_h_y_centro + ancho_pasillo / 2
pasillo_h_x_min = x_min - margen
pasillo_h_x_max = x_max + margen

# Pasillo vertical: sube por la derecha desde donde termina el horizontal
pasillo_v_x_max = x_max + margen
pasillo_v_x_min = pasillo_v_x_max - ancho_pasillo
pasillo_v_y_min = pasillo_h_y_max
pasillo_v_y_max = y_max + margen + 2.0  # un poco mas arriba para cubrir los puntos que faltaban

# Vertices de la L (6 puntos - dos rectangulos unidos)
l_base = np.array([
    [pasillo_h_x_min, pasillo_h_y_min],   # 1: Abajo-izquierda
    [pasillo_h_x_max, pasillo_h_y_min],   # 2: Abajo-derecha
    [pasillo_v_x_max, pasillo_v_y_max],   # 3: Arriba-derecha pasillo vertical
    [pasillo_v_x_min, pasillo_v_y_max],   # 4: Arriba-izquierda pasillo vertical
    [pasillo_v_x_min, pasillo_h_y_max],   # 5: Codo interior
    [pasillo_h_x_min, pasillo_h_y_max],   # 6: Izquierda arriba pasillo horizontal
], dtype=np.float32)

# Rotar ligeramente para que coincida con la inclinacion de los datos
angulo_deg = -10.0
angulo_rad = np.radians(angulo_deg)
centro_x = (x_min + x_max) / 2
centro_y = (y_min + y_max) / 2
cos_a = np.cos(angulo_rad)
sin_a = np.sin(angulo_rad)

l_vertices = np.zeros_like(l_base)
for i, (x, y) in enumerate(l_base):
    dx = x - centro_x
    dy = y - centro_y
    l_vertices[i, 0] = centro_x + dx * cos_a - dy * sin_a
    l_vertices[i, 1] = centro_y + dx * sin_a + dy * cos_a

print(f"\nL REAL (6 vertices - pasillo horizontal + vertical derecha):")
for i, (x, y) in enumerate(l_vertices):
    print(f"  V{i+1}: ({x:.2f}, {y:.2f})")

# Crear mapa - ALEJADO PARA VER LA FORMA L
px_por_metro = 30  # Escala pequeña para ver la L completa
margin = 4.0  # Margen grande para que se vea la forma

# Usar bounds de la L rotada para dimensionar el mapa
lv_xmin = l_vertices[:, 0].min()
lv_xmax = l_vertices[:, 0].max()
lv_ymin = l_vertices[:, 1].min()
lv_ymax = l_vertices[:, 1].max()

# Combinar con datos reales para asegurar que todo cabe
map_xmin = min(x_min, lv_xmin) - margin
map_xmax = max(x_max, lv_xmax) + margin
map_ymin = min(y_min, lv_ymin) - margin
map_ymax = max(y_max, lv_ymax) + margin

# Centrar la L en la imagen con margenes iguales
lv_cx = (lv_xmin + lv_xmax) / 2
lv_cy = (lv_ymin + lv_ymax) / 2
lv_hw = (lv_xmax - lv_xmin) / 2 + margin
lv_hh = (lv_ymax - lv_ymin) / 2 + margin
# Subir el centro para que el piso quede visualmente centrado
lv_cy = lv_cy + 2.0
map_xmin = lv_cx - lv_hw
map_xmax = lv_cx + lv_hw
map_ymin = lv_cy - lv_hh
map_ymax = lv_cy + lv_hh

nuevo_origen_x = map_xmin
nuevo_origen_y = map_ymax

ancho_px = int((map_xmax - map_xmin) * px_por_metro)
alto_px = int((map_ymax - map_ymin) * px_por_metro)

print(f"\nMapa:")
print(f"  Tamano: {ancho_px}x{alto_px} px")

img = np.full((alto_px, ancho_px, 3), (245, 245, 245), dtype=np.uint8)

# Funcion para convertir metros a pixels
def m2px(mx, my):
    px_x = int((mx - map_xmin) * px_por_metro)
    px_y = int((map_ymax - my) * px_por_metro)
    return px_x, px_y

origen_px = m2px(0, 0)

# Grilla
for x in range(origen_px[0] - px_por_metro, ancho_px, px_por_metro):
    cv2.line(img, (x, 0), (x, alto_px), (225, 225, 225), 1)
for y in range(origen_px[1] - px_por_metro, alto_px, px_por_metro):
    cv2.line(img, (0, y), (ancho_px, y), (225, 225, 225), 1)

for x in range(origen_px[0] - px_por_metro*5, ancho_px, px_por_metro*5):
    cv2.line(img, (x, 0), (x, alto_px), (190, 190, 190), 2)
for y in range(origen_px[1] - px_por_metro*5, alto_px, px_por_metro*5):
    cv2.line(img, (0, y), (ancho_px, y), (190, 190, 190), 2)

# Convertir L a pixels
vertices_px = []
for x, y in l_vertices:
    vertices_px.append(list(m2px(x, y)))

vertices_px = np.array(vertices_px, dtype=np.int32)

# Dibujar L REAL
cv2.fillPoly(img, [vertices_px], (150, 210, 180))
cv2.polylines(img, [vertices_px], True, (80, 160, 130), 3)

# Ejes
cv2.line(img, (origen_px[0], 0), (origen_px[0], alto_px), (255, 0, 0), 2)
cv2.line(img, (0, origen_px[1]), (ancho_px, origen_px[1]), (0, 0, 255), 2)

# Origen
cv2.circle(img, origen_px, 8, (0, 255, 0), -1)
cv2.putText(img, "(0,0)", (origen_px[0]+10, origen_px[1]-10),
            cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 0, 0), 1)

# Camaras
for cid in ['cam01', 'cam02', 'cam03']:
    cam = mapa['camaras'][cid]
    px_x, px_y = m2px(cam['posicion_m'][0], cam['posicion_m'][1])

    if 0 <= px_x < ancho_px and 0 <= px_y < alto_px:
        cv2.circle(img, (px_x, px_y), 8, (200, 0, 200), -1)
        cv2.putText(img, cid, (px_x+10, px_y),
                    cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 0, 0), 1)

# Trayectorias
for global_id in trajectory_df['global_id'].unique():
    persona = trajectory_df[trajectory_df['global_id'] == global_id]
    puntos_validos = persona[persona['x'].notna() & persona['y'].notna()].copy()

    if len(puntos_validos) > 0:
        px_x = ((puntos_validos['x'].values - map_xmin) * px_por_metro).astype(int)
        px_y = ((map_ymax - puntos_validos['y'].values) * px_por_metro).astype(int)

        valid = (px_x >= 0) & (px_x < ancho_px) & (px_y >= 0) & (px_y < alto_px)
        px_x = px_x[valid]
        px_y = px_y[valid]

        if len(px_x) > 1:
            puntos = np.column_stack([px_x, px_y])
            for i in range(len(puntos) - 1):
                cv2.line(img, tuple(puntos[i]), tuple(puntos[i+1]),
                        (0, 100, 255), 1)

        for px, py in zip(px_x, px_y):
            cv2.circle(img, (px, py), 2, (255, 100, 0), -1)

# Texto
cv2.putText(img, "PISO L - COBERTURA COMPLETA", (10, 30),
            cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 0, 0), 2)
cv2.putText(img, f"16 personas | {len(trajectory_df)} puntos | {px_por_metro} px/m",
            (10, 60), cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 0, 0), 1)

# Guardar
output_path = OUTPUT_DIR / "piso_L_simple.png"
cv2.imwrite(str(output_path), img)

print(f"\nPiso L simple guardado: {output_path}")
print(f"  Tamano: {img.shape[1]}x{img.shape[0]} px")
print("\n" + "=" * 80)
