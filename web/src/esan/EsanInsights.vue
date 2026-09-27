<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import PlanoEsan from "./PlanoEsan.vue";
import { api, colorPersona, GENEROS, segundos, TIPOS_ZONA, type Config, type Insights, type Sesion } from "./api";

const route = useRoute();
const config = ref<Config>();
const sesiones = ref<Sesion[]>([]);
const sesionId = ref("");
const datos = ref<Insights>();
const error = ref("");
const cargando = ref(true);

const mapa = computed(() => config.value?.map?.mapa);
const r = computed(() => datos.value?.resumen);
const pct = (n: number, total: number) => (total ? Math.round((100 * n) / total) : 0);

const ocupacion = computed(() => {
  const o = datos.value?.ocupacion;
  if (!o || !o.segundos.length) return null;
  const w = 600;
  const h = 150;
  const maxS = Math.max(1, o.segundos[o.segundos.length - 1]);
  const maxP = Math.max(1, ...o.personas);
  const x = (s: number) => (s / maxS) * w;
  const y = (p: number) => h - (p / maxP) * (h - 12);
  const linea = o.segundos.map((s, i) => `${x(s).toFixed(1)},${y(o.personas[i]).toFixed(1)}`).join(" ");
  const pico = o.personas.indexOf(maxP);
  const media = o.personas.reduce((a, b) => a + b, 0) / o.personas.length;
  return { w, h, linea, area: `0,${h} ${linea} ${w},${h}`, maxP, picoS: o.segundos[pico], picoX: x(o.segundos[pico]), picoY: y(maxP), media };
});

const permanencias = computed(() => {
  const ps = [...(datos.value?.personas ?? [])].sort((a, b) => b.permanencia_s - a.permanencia_s);
  const max = Math.max(1, ...ps.map((p) => p.permanencia_s));
  return ps.map((p) => ({ ...p, ancho: (100 * p.permanencia_s) / max }));
});

const maxCamara = computed(() => Math.max(1, ...(datos.value?.camaras ?? []).map((c) => c.personas)));

const insights = computed(() => {
  const d = datos.value;
  if (!d || !r.value) return [];
  const lista: string[] = [];
  const res = r.value;
  if (ocupacion.value)
    lista.push(`Pico de ocupación: ${ocupacion.value.maxP} personas a la vez, en el segundo ${ocupacion.value.picoS}; en promedio hubo ${ocupacion.value.media.toFixed(1)} personas simultáneas.`);
  if (res.personas)
    lista.push(`${res.multicamara} de ${res.personas} personas (${pct(res.multicamara, res.personas)} %) pasaron por más de una cámara: el Re-ID las siguió entre vistas.`);
  if (res.personas) {
    const mas = permanencias.value[0];
    lista.push(`Permanencia media de ${segundos(res.permanencia.media_s)} (mediana ${segundos(res.permanencia.mediana_s)}); la más larga fue G${mas.numero} con ${segundos(mas.permanencia_s)}.`);
  }
  const g = res.genero;
  const sin = g.SIN_DETERMINAR ?? 0;
  if (res.personas)
    lista.push(`Género estimado por el cuerpo: ${g.HOMBRE ?? 0} hombres y ${g.MUJER ?? 0} mujeres; ${sin} (${pct(sin, res.personas)} %) sin evidencia suficiente, sobre todo pasadas breves o lejanas.`);
  if (res.velocidad.media_mps != null)
    lista.push(`Caminan a ${res.velocidad.media_mps.toFixed(2)} m/s en promedio y están detenidos el ${Math.round(res.velocidad.detenidos_pct ?? 0)} % del tiempo: ${(res.velocidad.detenidos_pct ?? 0) > 35 ? "espacio de permanencia" : "espacio mayormente de paso"}.`);
  const cam = [...d.camaras].sort((a, b) => b.personas - a.personas)[0];
  if (cam) lista.push(`${cam.camera_id} es la cámara que más personas vio (${cam.personas}).`);
  if (d.flujos[0]) lista.push(`Flujo principal entre cámaras: ${d.flujos[0].desde} → ${d.flujos[0].hacia} (${d.flujos[0].personas} personas).`);
  const celda = [...d.calor.celdas].sort((a, b) => b[2] - a[2])[0];
  if (celda) lista.push(`El punto más concurrido del piso está en (${celda[0]}, ${celda[1]}) m, con ${celda[2]} segundos-persona.`);
  const zona = [...d.zonas].sort((a, b) => b.visitantes - a.visitantes)[0];
  if (zona?.visitantes)
    lista.push(`La zona más visitada es «${zona.name}»: ${zona.visitantes} personas, ${segundos(zona.permanencia_media_s)} de permanencia media.`);
  else if (!d.zonas.length) lista.push("Dibuja zonas en Configuración → ESAN para medir visitas y permanencia por área.");
  return lista;
});

async function cargar() {
  if (!sesionId.value) return;
  cargando.value = true;
  error.value = "";
  try {
    datos.value = await api.insights(sesionId.value);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    cargando.value = false;
  }
}
watch(sesionId, cargar);

onMounted(async () => {
  try {
    [config.value, sesiones.value] = await Promise.all([api.config(), api.sesiones()]);
    const pedida = typeof route.query.sesion === "string" ? route.query.sesion : "";
    sesionId.value = sesiones.value.find((s) => s.session_id === pedida)?.session_id ?? sesiones.value[0]?.session_id ?? "";
    if (!sesionId.value) cargando.value = false;
  } catch (e) {
    error.value = (e as Error).message;
    cargando.value = false;
  }
});
</script>

<template>
  <section class="page-title">
    <div>
      <p class="eyebrow">ESAN · INSIGHTS DEL MODELO</p>
      <h1>Qué pasó en la sesión</h1>
      <p>Calculado en PostgreSQL/PostGIS sobre las trayectorias guardadas por el modelo.</p>
    </div>
    <label class="selector-sesion">Sesión
      <select v-model="sesionId">
        <option v-for="s in sesiones" :key="s.session_id" :value="s.session_id">
          {{ s.name || s.session_id.slice(0, 8) }} · {{ s.kind === "BUILD" ? "dataset" : "teléfono" }} · {{ new Date(s.recording_start).toLocaleString() }}
        </option>
      </select>
    </label>
  </section>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <p v-else-if="!cargando && !sesiones.length" class="empty panel">Todavía no hay sesiones de ESAN en la base.</p>

  <template v-if="datos && r">
    <section class="kpis">
      <article class="panel metric"><p>PERSONAS</p><strong>{{ r.personas }}</strong><small>{{ r.multicamara }} en más de una cámara</small></article>
      <article class="panel metric"><p>DURACIÓN</p><strong>{{ segundos(r.duracion_s) }}</strong><small>{{ r.puntos.toLocaleString() }} puntos de trayectoria</small></article>
      <article class="panel metric"><p>PERMANENCIA MEDIA</p><strong>{{ segundos(r.permanencia.media_s) }}</strong><small>mediana {{ segundos(r.permanencia.mediana_s) }} · máx {{ segundos(r.permanencia.max_s) }}</small></article>
      <article class="panel metric"><p>VELOCIDAD AL CAMINAR</p><strong>{{ r.velocidad.media_mps != null ? r.velocidad.media_mps.toFixed(2) + " m/s" : "—" }}</strong><small>{{ r.velocidad.detenidos_pct != null ? Math.round(r.velocidad.detenidos_pct) + " % del tiempo detenidos" : "sin posiciones en el plano" }}</small></article>
    </section>

    <section class="panel insights-texto">
      <div class="panel-heading"><h2>Insights</h2><span class="pill">generados con los datos de la sesión</span></div>
      <ul>
        <li v-for="(texto, i) in insights" :key="i">{{ texto }}</li>
      </ul>
    </section>

    <div class="grilla">
      <article class="panel">
        <div class="panel-heading"><h2>Ocupación en el tiempo</h2><span class="muted">personas visibles por segundo</span></div>
        <svg v-if="ocupacion" class="grafico" :viewBox="`0 0 ${ocupacion.w} ${ocupacion.h}`" preserveAspectRatio="none">
          <polygon :points="ocupacion.area" class="area" />
          <polyline :points="ocupacion.linea" class="linea" />
          <circle :cx="ocupacion.picoX" :cy="ocupacion.picoY" r="4" class="pico" />
        </svg>
        <p v-if="ocupacion" class="chart-caption">Pico: {{ ocupacion.maxP }} personas en el segundo {{ ocupacion.picoS }}.</p>
      </article>

      <article class="panel">
        <div class="panel-heading"><h2>Género estimado</h2><span class="muted">por cuerpo, sin rostro</span></div>
        <div class="barra-genero">
          <span v-for="(n, g) in r.genero" :key="g" :class="'g-' + g" :style="{ flex: n || 0.0001 }" :title="`${GENEROS[g]}: ${n}`">{{ n ? n : "" }}</span>
        </div>
        <ul class="leyenda">
          <li v-for="(n, g) in r.genero" :key="g"><span :class="'marca g-' + g"></span>{{ GENEROS[g] }} · {{ n }} ({{ pct(n, r.personas) }} %)</li>
        </ul>
      </article>

      <article class="panel">
        <div class="panel-heading"><h2>Cámaras y flujos</h2><span class="muted">personas distintas</span></div>
        <ul class="barras">
          <li v-for="c in datos.camaras" :key="c.camera_id">
            <span>{{ c.camera_id }}</span>
            <span class="pista"><span class="relleno" :style="{ width: (100 * c.personas) / maxCamara + '%' }"></span></span>
            <b>{{ c.personas }}</b>
          </li>
        </ul>
        <p v-if="datos.flujos.length" class="chart-caption">
          <template v-for="(f, i) in datos.flujos" :key="i">{{ i ? " · " : "" }}{{ f.desde }} → {{ f.hacia }}: {{ f.personas }}</template>
        </p>
      </article>

      <article class="panel">
        <div class="panel-heading"><h2>Permanencia por persona</h2><span class="muted">desde que aparece hasta que sale</span></div>
        <ul class="barras permanencias">
          <li v-for="p in permanencias" :key="p.numero">
            <span><span class="punto" :style="{ background: colorPersona(p.numero) }"></span>G{{ p.numero }}</span>
            <span class="pista"><span class="relleno" :style="{ width: p.ancho + '%', background: colorPersona(p.numero) }"></span></span>
            <b>{{ segundos(p.permanencia_s) }}</b>
          </li>
        </ul>
      </article>
    </div>

    <div class="grilla dos">
      <article v-if="mapa" class="panel">
        <div class="panel-heading"><h2>Mapa de calor</h2><span class="muted">segundos-persona por m²</span></div>
        <PlanoEsan class="calor" :mapa="mapa" :zonas="config?.zones ?? []" :calor="datos.calor.celdas" :mostrar-camaras="false" />
        <p v-if="!datos.calor.celdas.length" class="chart-caption">Esta sesión no tiene posiciones en el plano (la cámara del teléfono no está calibrada al piso).</p>
      </article>
      <article class="panel">
        <div class="panel-heading"><h2>Zonas</h2><RouterLink to="/configuracion/esan" class="muted">Editar zonas →</RouterLink></div>
        <table v-if="datos.zonas.length" class="tabla">
          <thead><tr><th>Zona</th><th>Tipo</th><th>Área</th><th>Visitantes</th><th>Permanencia media</th></tr></thead>
          <tbody>
            <tr v-for="z in datos.zonas" :key="z.zone_id">
              <td>{{ z.name }}</td><td>{{ TIPOS_ZONA[z.zone_type] ?? z.zone_type }}</td><td>{{ z.area_m2.toFixed(1) }} m²</td>
              <td>{{ z.visitantes }}</td><td>{{ segundos(z.permanencia_media_s) }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="empty">Sin zonas. Dibújalas en Configuración → ESAN y aquí verás visitas y permanencia por área.</p>
      </article>
    </div>
  </template>
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
.kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}
.insights-texto ul {
  margin: 0;
  padding: 12px 16px 14px 32px;
  display: grid;
  gap: 7px;
  font-size: 12.5px;
  line-height: 1.45;
}
.grilla {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin-top: 14px;
}
.grafico {
  display: block;
  width: calc(100% - 24px);
  height: 150px;
  margin: 12px;
}
.grafico .area {
  fill: rgba(61, 139, 255, 0.16);
}
.grafico .linea {
  fill: none;
  stroke: var(--blue-600);
  stroke-width: 2;
  vector-effect: non-scaling-stroke;
}
.grafico .pico {
  fill: var(--danger);
}
.barra-genero {
  display: flex;
  height: 30px;
  margin: 14px;
  border-radius: 8px;
  overflow: hidden;
}
.barra-genero span {
  display: grid;
  place-items: center;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}
.g-HOMBRE {
  background: #3d8bff;
}
.g-MUJER {
  background: #d9468f;
}
.g-SIN_DETERMINAR {
  background: #8ba4bf;
}
.leyenda {
  list-style: none;
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  padding: 0 14px 14px;
  margin: 0;
  font-size: 11.5px;
}
.marca {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 3px;
  margin-right: 5px;
}
.barras {
  list-style: none;
  margin: 0;
  padding: 12px 14px;
  display: grid;
  gap: 8px;
  font-size: 11.5px;
}
.barras li {
  display: grid;
  grid-template-columns: 64px 1fr 70px;
  align-items: center;
  gap: 8px;
}
.barras b {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.permanencias {
  max-height: 300px;
  overflow: auto;
}
.pista {
  height: 10px;
  border-radius: 6px;
  background: rgba(139, 164, 191, 0.2);
  overflow: hidden;
}
.relleno {
  display: block;
  height: 100%;
  border-radius: 6px;
  background: var(--blue-600);
}
.punto {
  display: inline-block;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  margin-right: 5px;
}
.calor {
  margin: 10px;
  width: calc(100% - 20px);
}
.tabla {
  width: calc(100% - 24px);
  margin: 12px;
  border-collapse: collapse;
  font-size: 11.5px;
}
.tabla th,
.tabla td {
  padding: 7px 6px;
  text-align: left;
  border-bottom: 1px solid var(--glass-line);
}
@media (max-width: 1100px) {
  .kpis,
  .grilla {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 700px) {
  .kpis,
  .grilla {
    grid-template-columns: 1fr;
  }
}
</style>
