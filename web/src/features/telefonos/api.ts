import { json, request } from "../../core/http";

export type Telefono = { id: string; nombre: string; url: string };

export type EstadoTelefono = {
  nombre: string;
  fuente: string;
  estado: "procesando" | "conectando" | "sin_conexion";
  mensaje: string | null;
  fps?: number;
  latencia_ms?: number | null;
  saltados?: number;
  personas_ahora?: number;
};

/** Estado que publica el servicio del modelo (Modelo/Test Modelo/camara_telefono.py). */
export type EstadoServicio = {
  ts: number;
  estado: "procesando" | "sin_telefonos" | "detenido";
  dispositivo?: string;
  segundos?: number;
  personas_total?: number;
  multitelefono?: number;
  genero?: Record<string, number>;
  telefonos: Record<string, EstadoTelefono>;
};

export const api = {
  telefonos: () => request<Telefono[]>("/api/v1/telefonos"),
  agregar: (datos: { nombre: string; url: string }) => request<Telefono>("/api/v1/telefonos", json("POST", datos)),
  quitar: (id: string) => request<void>(`/api/v1/telefonos/${encodeURIComponent(id)}`, { method: "DELETE" }),
};
