export type Punto = [number, number];

export type PoseCamara = {
  posicion_m: Punto;
  direccion: Punto;
  altura_m: number;
  focal_px: number;
};

export type Mapa = {
  px_por_metro: number;
  origen_m: Punto;
  tam_px: [number, number];
  desfases_s: Record<string, number>;
  camaras: Record<string, PoseCamara>;
  piso_L_m?: Punto[];
  informe?: Record<string, Record<string, number>>;
};

export type ConfigCamaras = {
  mode: string;
  overlaps?: [string, string][];
  cameras: Record<string, { timestamp_offset?: number; resolution?: [number, number]; principal_point?: Punto }>;
};

export type Camara = {
  camera_id: string;
  name: string;
  stream_uri: string;
  fps: number | null;
  width_px: number | null;
  height_px: number | null;
  timestamp_offset_s: number;
  position: Punto | null;
  angle_deg: number | null;
  homography: number[] | null;
  active: boolean;
};

export type Zona = {
  zone_id: number;
  name: string;
  zone_type: string;
  color: string;
  points: Punto[];
  area_m2: number;
};

export type Config = {
  floor_id: number;
  map: { mapa: Mapa; camaras: ConfigCamaras } | null;
  cameras: Camara[];
  zones: Zona[];
};

export type Sesion = {
  session_id: string;
  name: string;
  kind: "BUILD" | "LIVE";
  status: string;
  recording_start: string;
  ended_at: string | null;
  summary: Record<string, unknown> | null;
  identities: number;
  points: number;
  duration_s: number;
};

export type Recorrido = {
  numero: number;
  genero: string;
  confianza: number | null;
  camaras: string[];
  inicio_s: number;
  fin_s: number;
  k: number[];
  x: number[];
  y: number[];
};

export type Replay = { session: Sesion; paso_s: number; duracion_s: number; personas: Recorrido[] };

export type Insights = {
  session: Sesion;
  resumen: {
    personas: number;
    multicamara: number;
    puntos: number;
    duracion_s: number | null;
    genero: Record<string, number>;
    permanencia: { media_s: number; mediana_s: number; max_s: number };
    velocidad: { media_mps: number | null; mediana_mps: number | null; detenidos_pct: number | null };
  };
  personas: { numero: number; genero: string; confianza: number | null; camaras: number; permanencia_s: number; inicio_s: number }[];
  ocupacion: { segundos: number[]; personas: number[] };
  camaras: { camera_id: string; personas: number; puntos: number }[];
  flujos: { desde: string; hacia: string; personas: number }[];
  calor: { celda_m: number; celdas: [number, number, number][] };
  zonas: { zone_id: number; name: string; zone_type: string; area_m2: number; visitantes: number; segundos_persona: number; permanencia_media_s: number }[];
};

export const TIPOS_ZONA: Record<string, string> = {
  PASILLO: "Pasillo",
  ENTRADA: "Entrada",
  COLA: "Cola",
  CHECKIN: "Atención",
  SEGURIDAD: "Control",
  PUERTA: "Puerta",
  OTRO: "Otro",
};

export const GENEROS: Record<string, string> = { HOMBRE: "Hombre", MUJER: "Mujer", SIN_DETERMINAR: "Sin determinar" };

async function pedir<T>(url: string, init?: RequestInit): Promise<T> {
  const r = await fetch(url, { ...init, headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) } });
  if (!r.ok) {
    const cuerpo = await r.json().catch(() => ({}));
    throw new Error((cuerpo as { error?: string }).error ?? `Error ${r.status}`);
  }
  return r.status === 204 ? (undefined as T) : ((await r.json()) as T);
}

export const api = {
  config: () => pedir<Config>("/api/v1/esan/config"),
  sesiones: () => pedir<Sesion[]>("/api/v1/esan/sessions"),
  replay: (id: string) => pedir<Replay>(`/api/v1/esan/sessions/${id}/replay`),
  insights: (id: string) => pedir<Insights>(`/api/v1/esan/sessions/${id}/insights`),
  guardarCamara: (id: string, datos: { name: string; stream_uri: string; active?: boolean }) =>
    pedir<Camara>(`/api/v1/esan/cameras/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(datos) }),
  guardarZona: (datos: { name: string; zone_type: string; color: string; points: Punto[] }, id?: number) =>
    pedir<Zona>(id ? `/api/v1/esan/zones/${id}` : "/api/v1/esan/zones", { method: id ? "PUT" : "POST", body: JSON.stringify(datos) }),
  borrarZona: (id: number) => pedir<void>(`/api/v1/esan/zones/${id}`, { method: "DELETE" }),
};

export function colorPersona(numero: number): string {
  return `hsl(${Math.round(((numero * 0.618034) % 1) * 360)}, 78%, 48%)`;
}

export function segundos(s: number | null | undefined): string {
  if (s == null) return "—";
  if (s < 60) return `${s.toFixed(1)} s`;
  return `${Math.floor(s / 60)} min ${Math.round(s % 60)} s`;
}

export function wsUrl(ruta: string): string {
  return `${location.protocol === "https:" ? "wss:" : "ws:"}//${location.host}${ruta}`;
}
