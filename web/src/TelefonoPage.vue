<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { api, colorPersona, wsUrl } from "./esan/api";

type Persona = {
  id: number;
  global_id: number | null;
  local_id: number;
  box: [number, number, number, number];
  conf: number;
  gender: string | null;
  gender_conf: number | null;
};
type Estado = {
  estado: "procesando" | "sin_fuente" | "sin_conexion" | "detenido";
  mensaje?: string;
  fuente?: string;
  sesion?: string;
  fps?: number;
  latencia_ms?: number;
  saltados?: number;
  personas_ahora?: number;
  personas_total?: number;
  segundos?: number;
  dispositivo?: string;
};
type Mensaje = { ts: number; frame_w: number; frame_h: number; people: Persona[]; estado?: Estado };

const CAMARA = "esan-movil";
const url = ref("");
const guardada = ref("");
const error = ref("");
const aviso = ref("");
const imagen = ref("");
const lienzo = ref<HTMLCanvasElement>();
const ultimo = ref<Mensaje>();
const ultimoMensaje = ref(0);
const ahora = ref(Date.now());
let video: WebSocket | undefined;
let detecciones: WebSocket | undefined;
let reintentos: ReturnType<typeof setTimeout>[] = [];
let reloj: ReturnType<typeof setInterval> | undefined;
let activo = true;

const estado = computed(() => ultimo.value?.estado);
const conectado = computed(() => ahora.value - ultimoMensaje.value < 6000);
const textoEstado = computed(() => {
  if (!conectado.value) return { clase: "warn", texto: "El servicio del modelo no está corriendo (paso 3)." };
  switch (estado.value?.estado) {
    case "procesando":
      return { clase: "good", texto: `Procesando ${estado.value.fuente ?? ""} en ${estado.value.dispositivo ?? ""}` };
    case "sin_fuente":
      return { clase: "warn", texto: estado.value.mensaje ?? "Falta la URL del teléfono." };
    case "sin_conexion":
      return { clase: "warn", texto: estado.value.mensaje ?? "No se pudo leer la cámara del teléfono." };
    default:
      return { clase: "muted", texto: estado.value?.mensaje ?? "Servicio detenido." };
  }
});

function dibujar() {
  const c = lienzo.value;
  const m = ultimo.value;
  if (!c || !m || !m.frame_w) return;
  if (c.width !== m.frame_w || c.height !== m.frame_h) {
    c.width = m.frame_w;
    c.height = m.frame_h;
  }
  const ctx = c.getContext("2d");
  if (!ctx) return;
  ctx.clearRect(0, 0, c.width, c.height);
  ctx.lineWidth = 2;
  ctx.font = "600 13px system-ui, sans-serif";
  for (const p of m.people) {
    const [x1, y1, x2, y2] = p.box;
    const color = p.global_id != null ? colorPersona(p.global_id) : "#8ba4bf";
    ctx.strokeStyle = color;
    ctx.strokeRect(x1, y1, x2 - x1, y2 - y1);
    const texto = `${p.global_id != null ? "G" + p.global_id : "L" + p.local_id}${p.gender ? " · " + p.gender + (p.gender_conf ? " " + Math.round(p.gender_conf * 100) + "%" : "") : ""}`;
    const ancho = ctx.measureText(texto).width + 10;
    const y = Math.max(18, y1);
    ctx.fillStyle = "rgba(10, 20, 35, 0.82)";
    ctx.fillRect(x1, y - 18, ancho, 18);
    ctx.fillStyle = color;
    ctx.fillText(texto, x1 + 5, y - 5);
  }
}

function conectar(ruta: string, binario: boolean, alRecibir: (ev: MessageEvent) => void): WebSocket {
  const s = new WebSocket(wsUrl(ruta));
  if (binario) s.binaryType = "arraybuffer";
  s.addEventListener("message", alRecibir);
  s.addEventListener("close", () => {
    if (!activo) return;
    reintentos.push(setTimeout(() => {
      if (ruta.includes("detections")) detecciones = conectar(ruta, binario, alRecibir);
      else video = conectar(ruta, binario, alRecibir);
    }, 2000));
  });
  s.addEventListener("error", () => s.close());
  return s;
}

function alVideo(ev: MessageEvent) {
  const nueva = URL.createObjectURL(new Blob([ev.data as ArrayBuffer], { type: "image/jpeg" }));
  const vieja = imagen.value;
  imagen.value = nueva;
  if (vieja) URL.revokeObjectURL(vieja);
}

function alDetecciones(ev: MessageEvent) {
  try {
    ultimo.value = JSON.parse(ev.data as string) as Mensaje;
    ultimoMensaje.value = Date.now();
    requestAnimationFrame(dibujar);
  } catch {
    return;
  }
}

async function guardarUrl() {
  error.value = aviso.value = "";
  try {
    await api.guardarCamara(CAMARA, { name: "Cámara del teléfono", stream_uri: url.value.trim() });
    guardada.value = url.value.trim();
    aviso.value = url.value.trim() ? "URL guardada: el servicio del modelo la toma en unos segundos." : "URL borrada.";
  } catch (e) {
    error.value = (e as Error).message;
  }
}

watch(imagen, () => requestAnimationFrame(dibujar));

onMounted(async () => {
  try {
    const cfg = await api.config();
    guardada.value = url.value = cfg.cameras.find((c) => c.camera_id === CAMARA)?.stream_uri ?? "";
  } catch (e) {
    error.value = (e as Error).message;
  }
  video = conectar(`/api/v1/cameras/${CAMARA}/watch`, true, alVideo);
  detecciones = conectar(`/api/v1/cameras/${CAMARA}/detections/watch`, false, alDetecciones);
  reloj = setInterval(() => (ahora.value = Date.now()), 1000);
});

onUnmounted(() => {
  activo = false;
  reintentos.forEach(clearTimeout);
  clearInterval(reloj);
  video?.close();
  detecciones?.close();
  if (imagen.value) URL.revokeObjectURL(imagen.value);
});
</script>

<template>
  <section class="page-title">
    <div>
      <p class="eyebrow">MODELO FINAL · CÁMARA DEL TELÉFONO</p>
      <h1>Tracking en vivo con tu teléfono</h1>
      <p>La app convierte el teléfono en cámara IP; el modelo final (YOLO26m + tracker + Re-ID + género) corre en la GPU de esta laptop y guarda la sesión en la base.</p>
    </div>
    <span :class="textoEstado.clase" class="estado-servicio">● {{ textoEstado.texto }}</span>
  </section>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <p v-if="aviso" class="success">{{ aviso }}</p>

  <div class="telefono">
    <section class="panel en-vivo">
      <div class="panel-heading">
        <h2>Cámara del teléfono</h2>
        <span class="heading-meta">
          <span class="pill">{{ ultimo?.people.length ?? 0 }} en cuadro</span>
          <RouterLink v-if="estado?.sesion" class="pill" :to="`/insights/esan?sesion=${estado.sesion}`">Insights de esta sesión →</RouterLink>
        </span>
      </div>
      <div class="lienzo">
        <img v-if="imagen && conectado" :src="imagen" alt="Cámara del teléfono en vivo" />
        <canvas v-show="imagen && conectado" ref="lienzo"></canvas>
        <p v-if="!imagen || !conectado" class="espera">Sin video todavía. Sigue los pasos de la derecha.</p>
      </div>
      <div v-if="estado?.estado === 'procesando'" class="stats">
        <div><b>{{ estado.fps }}</b><small>FPS procesados</small></div>
        <div><b>{{ estado.latencia_ms }} ms</b><small>latencia</small></div>
        <div><b>{{ estado.personas_ahora }}</b><small>personas ahora</small></div>
        <div><b>{{ estado.personas_total }}</b><small>personas en la sesión</small></div>
        <div><b>{{ estado.saltados }}</b><small>frames saltados</small></div>
        <div><b>{{ Math.round(estado.segundos ?? 0) }} s</b><small>de sesión</small></div>
      </div>
    </section>

    <aside class="panel pasos">
      <div class="panel-heading"><h2>Cómo conectarlo</h2></div>
      <ol>
        <li>
          <b>Instala la app en el teléfono Android.</b>
          <a class="button" href="/descargas/CamaraESAN.apk" download>⬇ CamaraESAN.apk</a>
          <small class="muted">Ábrela, concede la cámara y pulsa <b>Transmitir</b>. Teléfono y laptop en la misma red Wi-Fi (o el puerto 8080 del teléfono publicado a internet).</small>
        </li>
        <li>
          <b>Pega la URL que muestra la app.</b>
          <input v-model="url" placeholder="http://192.168.1.50:8080/video?token=…" />
          <button class="primary-button" type="button" :disabled="url.trim() === guardada" @click="guardarUrl">Guardar URL</button>
        </li>
        <li>
          <b>Inicia el modelo en la laptop (GPU).</b>
          <code>python "Modelo/Test Modelo/camara_telefono.py"</code>
          <small class="muted">Sin teléfono a mano: <code>python "Modelo/Test Modelo/simular_telefono.py"</code> y la URL <code>http://127.0.0.1:8090/video?token=prueba</code>.</small>
        </li>
        <li><b>Mira el tracking aquí.</b> La sesión se guarda cada 15 s y aparece en Insights → ESAN.</li>
      </ol>
    </aside>
  </div>
</template>

<style scoped>
.estado-servicio {
  font-size: 11.5px;
}
.telefono {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 14px;
  align-items: start;
}
.lienzo {
  position: relative;
  margin: 10px;
  min-height: 240px;
  border-radius: var(--radius-s);
  overflow: hidden;
  background: #0a1422;
}
.lienzo img,
.lienzo canvas {
  display: block;
  width: 100%;
  height: auto;
}
.lienzo canvas {
  position: absolute;
  inset: 0;
  height: 100%;
}
.espera {
  display: grid;
  place-items: center;
  min-height: 240px;
  margin: 0;
  color: #a9c0dc;
  font-size: 12px;
}
.stats {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 8px;
  padding: 0 12px 12px;
}
.stats div {
  display: flex;
  flex-direction: column;
  padding: 8px;
  border-radius: 8px;
  background: var(--pill-bg);
}
.stats b {
  font-size: 17px;
  font-variant-numeric: tabular-nums;
}
.stats small {
  font-size: 9.5px;
  color: var(--ink-soft);
}
.pasos ol {
  margin: 0;
  padding: 12px 14px 14px 32px;
  display: grid;
  gap: 14px;
  font-size: 12px;
}
.pasos li {
  display: grid;
  gap: 6px;
}
.pasos code {
  font-size: 10.5px;
  word-break: break-all;
}
@media (max-width: 1100px) {
  .telefono {
    grid-template-columns: 1fr;
  }
  .stats {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
