<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import PlanoEsan, { type PersonaPlano } from "./PlanoEsan.vue";
import { api, colorPersona, GENEROS, segundos, type Config, type Punto, type Replay, type Sesion } from "./api";

const config = ref<Config>();
const sesiones = ref<Sesion[]>([]);
const sesionId = ref("");
const replay = ref<Replay>();
const error = ref("");
const cargando = ref(true);
const t = ref(0);
const reproduciendo = ref(false);
const velocidad = ref(1);
const videos = ref<Record<string, HTMLVideoElement>>({});
const RASTRO_S = 3;
let cuadro = 0;
let anterior = 0;
let ultimaSincronia = 0;

const sesion = computed(() => sesiones.value.find((s) => s.session_id === sesionId.value));
const mapa = computed(() => config.value?.map?.mapa);
const desfases = computed(() => mapa.value?.desfases_s ?? {});
const camarasVideo = computed(() =>
  sesion.value?.kind === "BUILD" ? Object.keys(desfases.value).sort() : [],
);
const duracion = computed(() => replay.value?.duracion_s ?? 0);

function posicion(p: Replay["personas"][number], instante: number): { x: number; y: number } | null {
  const paso = replay.value?.paso_s ?? 0.2;
  const objetivo = instante / paso;
  let lo = 0;
  let hi = p.k.length - 1;
  if (hi < 0 || objetivo < p.k[0] - 2 || objetivo > p.k[hi] + 2) return null;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (p.k[mid] <= objetivo) lo = mid;
    else hi = mid - 1;
  }
  const i = lo;
  if (Math.abs(p.k[i] - objetivo) * paso > 1 && (i + 1 >= p.k.length || (p.k[i + 1] - objetivo) * paso > 1)) return null;
  if (i + 1 < p.k.length && p.k[i] <= objetivo && p.k[i + 1] - p.k[i] <= 5) {
    const f = (objetivo - p.k[i]) / (p.k[i + 1] - p.k[i]);
    return { x: p.x[i] + f * (p.x[i + 1] - p.x[i]), y: p.y[i] + f * (p.y[i + 1] - p.y[i]) };
  }
  return { x: p.x[i], y: p.y[i] };
}

const personas = computed<PersonaPlano[]>(() => {
  if (!replay.value) return [];
  const paso = replay.value.paso_s;
  return replay.value.personas.flatMap((p) => {
    const actual = posicion(p, t.value);
    if (!actual) return [];
    const rastro: Punto[] = [];
    for (let i = 0; i < p.k.length; i++) {
      const s = p.k[i] * paso;
      if (s > t.value) break;
      if (s >= t.value - RASTRO_S) rastro.push([p.x[i], p.y[i]]);
    }
    rastro.push([actual.x, actual.y]);
    return [{ id: p.numero, x: actual.x, y: actual.y, color: colorPersona(p.numero), etiqueta: `G${p.numero}`, rastro }];
  });
});

const presentes = computed(() => {
  const ids = new Set(personas.value.map((p) => p.id));
  return (replay.value?.personas ?? []).filter((p) => ids.has(p.numero));
});

async function cargarSesion() {
  if (!sesionId.value) return;
  pausar();
  cargando.value = true;
  error.value = "";
  try {
    replay.value = await api.replay(sesionId.value);
    t.value = 0;
    sincronizar(true);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    cargando.value = false;
  }
}

function tiempoVideo(cid: string) {
  return t.value - (desfases.value[cid] ?? 0);
}

function sincronizar(forzar = false) {
  for (const cid of camarasVideo.value) {
    const v = videos.value[cid];
    if (!v) continue;
    const objetivo = tiempoVideo(cid);
    if (objetivo < 0 || (v.duration && objetivo > v.duration)) {
      v.pause();
      if (forzar || v.currentTime !== Math.max(0, Math.min(objetivo, v.duration || 0))) v.currentTime = Math.max(0, Math.min(objetivo, v.duration || 0));
      continue;
    }
    v.playbackRate = velocidad.value;
    if (forzar || Math.abs(v.currentTime - objetivo) > 0.3) v.currentTime = objetivo;
    if (reproduciendo.value && v.paused) v.play().catch(() => undefined);
    if (!reproduciendo.value && !v.paused) v.pause();
  }
}

function avanzar(ahora: number) {
  const dt = anterior ? (ahora - anterior) / 1000 : 0;
  anterior = ahora;
  t.value = Math.min(duracion.value, t.value + dt * velocidad.value);
  if (ahora - ultimaSincronia > 500) {
    ultimaSincronia = ahora;
    sincronizar();
  }
  if (t.value >= duracion.value) pausar();
  else cuadro = requestAnimationFrame(avanzar);
}

function reproducir() {
  if (!replay.value) return;
  if (t.value >= duracion.value) t.value = 0;
  reproduciendo.value = true;
  anterior = 0;
  sincronizar(true);
  cuadro = requestAnimationFrame(avanzar);
}

function pausar() {
  reproduciendo.value = false;
  cancelAnimationFrame(cuadro);
  sincronizar();
}

function buscar(ev: Event) {
  t.value = Number((ev.target as HTMLInputElement).value);
  sincronizar(true);
}

watch(velocidad, () => sincronizar(true));
watch(sesionId, cargarSesion);

onMounted(async () => {
  try {
    [config.value, sesiones.value] = await Promise.all([api.config(), api.sesiones()]);
    const build = sesiones.value.find((s) => s.kind === "BUILD" && s.points > 0) ?? sesiones.value[0];
    if (build) sesionId.value = build.session_id;
    else cargando.value = false;
  } catch (e) {
    error.value = (e as Error).message;
    cargando.value = false;
  }
});
onUnmounted(() => cancelAnimationFrame(cuadro));
</script>

<template>
  <section class="page-title">
    <div>
      <p class="eyebrow">ESAN · DEMO EN VIVO DEL MODELO</p>
      <h1>Personas sobre el plano y cámaras sincronizadas</h1>
      <p>Reproduce una sesión guardada en la base: posiciones en metros, IDs globales y los videos procesados, en el mismo reloj.</p>
    </div>
    <label class="selector-sesion">Sesión
      <select v-model="sesionId">
        <option v-for="s in sesiones" :key="s.session_id" :value="s.session_id">
          {{ s.name || s.session_id.slice(0, 8) }} · {{ s.kind === "BUILD" ? "dataset" : "teléfono" }} · {{ s.identities }} personas
        </option>
      </select>
    </label>
  </section>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <p v-else-if="!cargando && !sesiones.length" class="empty panel">
    Todavía no hay sesiones de ESAN en la base. Ejecuta <code>Build Modelo/Build_Modelo.ipynb</code> (sección «Publicar en la base de datos»).
  </p>

  <div v-if="mapa && replay" class="envivo-esan">
    <section class="panel plano-panel">
      <div class="panel-heading">
        <h2>Plano ESAN · {{ (mapa.tam_px[0] / mapa.px_por_metro).toFixed(0) }} × {{ (mapa.tam_px[1] / mapa.px_por_metro).toFixed(0) }} m</h2>
        <span class="heading-meta"><span class="pill">{{ personas.length }} en el plano</span><span class="pill">{{ replay.personas.length }} personas</span></span>
      </div>
      <PlanoEsan :mapa="mapa" :zonas="config?.zones ?? []" :personas="personas" />
      <div class="controles">
        <button class="primary-button" type="button" @click="reproduciendo ? pausar() : reproducir()">{{ reproduciendo ? "❚❚ Pausa" : "▶ Reproducir" }}</button>
        <button type="button" @click="t = 0; sincronizar(true)">↺</button>
        <input class="linea-tiempo" type="range" min="0" :max="duracion" step="0.05" :value="t" @input="buscar" aria-label="Tiempo de la sesión" />
        <span class="reloj">{{ t.toFixed(1) }} / {{ duracion.toFixed(1) }} s</span>
        <select v-model.number="velocidad" aria-label="Velocidad">
          <option :value="0.5">0,5×</option><option :value="1">1×</option><option :value="2">2×</option><option :value="4">4×</option>
        </select>
      </div>
    </section>

    <aside class="panel presentes">
      <div class="panel-heading"><h2>En este instante</h2><span class="pill">{{ presentes.length }}</span></div>
      <ul>
        <li v-for="p in presentes" :key="p.numero">
          <span class="punto" :style="{ background: colorPersona(p.numero) }"></span>
          <b>G{{ p.numero }}</b>
          <span>{{ GENEROS[p.genero] ?? p.genero }}<template v-if="p.confianza"> · {{ Math.round(p.confianza * 100) }}%</template></span>
          <small class="muted">{{ p.camaras.join(", ") }} · {{ segundos(p.fin_s - p.inicio_s) }}</small>
        </li>
        <li v-if="!presentes.length" class="muted">Nadie en el plano en este instante.</li>
      </ul>
    </aside>
  </div>

  <section v-if="replay && camarasVideo.length" class="videos-esan">
    <article v-for="cid in camarasVideo" :key="cid" class="panel">
      <div class="panel-heading">
        <h2>{{ cid }}</h2>
        <span class="muted">desfase {{ (desfases[cid] ?? 0).toFixed(1) }} s</span>
      </div>
      <video
        :ref="(el) => { if (el) videos[cid] = el as HTMLVideoElement }"
        :src="`/api/esan/video/${cid}_procesado.mp4`"
        muted
        playsinline
        preload="auto"
      ></video>
      <a class="descarga" :href="`/api/esan/video/${cid}_procesado.mp4`" download>⬇ Descargar</a>
    </article>
  </section>
  <p v-if="replay && sesion?.kind === 'BUILD'" class="descargas muted">
    Resultados del build:
    <a href="/api/esan/video/mapa_2d.mp4" download>mapa_2d.mp4</a> ·
    <a href="/api/esan/video/mapa_trayectorias.png" download>mapa_trayectorias.png</a> ·
    <a href="/api/esan/video/trajectory_points.csv" download>trajectory_points.csv</a> ·
    <a href="/api/esan/video/identidades_genero.csv" download>identidades_genero.csv</a>
  </p>
</template>

<style scoped>
.selector-sesion {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 10.5px;
  font-weight: 650;
  color: var(--ink-soft);
}
.envivo-esan {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 260px;
  gap: 14px;
  align-items: start;
}
.plano-panel :deep(.plano-esan) {
  margin: 10px;
  width: calc(100% - 20px);
}
.controles {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px 12px;
}
.linea-tiempo {
  flex: 1 1 auto;
  width: auto;
  min-width: 160px;
}
.controles select {
  width: auto;
  flex: none;
}
.reloj {
  font-variant-numeric: tabular-nums;
  font-size: 11px;
  min-width: 96px;
  text-align: right;
}
.presentes ul {
  list-style: none;
  margin: 0;
  padding: 8px 12px;
  display: grid;
  gap: 8px;
  max-height: 520px;
  overflow: auto;
  font-size: 11.5px;
}
.presentes li {
  display: grid;
  grid-template-columns: 12px 36px 1fr;
  align-items: center;
  gap: 2px 6px;
}
.presentes small {
  grid-column: 2 / 4;
}
.punto {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.videos-esan {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 14px;
  margin-top: 14px;
}
.videos-esan video {
  display: block;
  width: 100%;
  background: #000;
}
.descarga {
  display: inline-block;
  padding: 8px 12px;
  font-size: 11px;
}
.descargas {
  margin-top: 12px;
}
@media (max-width: 980px) {
  .envivo-esan {
    grid-template-columns: 1fr;
  }
}
</style>
