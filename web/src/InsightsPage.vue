<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import LocalPlan from "./LocalPlan.vue";

type Metrics = {
  active: number;
  density: number;
  visits: number;
  pass_by: number;
  dwell_seconds: number | null;
  exposed: number;
  captured: number;
  capture_rate: number | null;
};
type Zone = { id: string; name: string; kind: string; area_m2: number };
type Summary = {
  from: string;
  to: string;
  zones: Zone[];
  data_through: string | null;
  unique: number;
  complete: number;
  censored: number;
  series: { at: string; visits: number }[];
  metrics: Metrics;
};
type Spatial = {
  routes: { id: string; points: { x: number; y: number }[] }[];
  heat: { x: number; y: number; seconds: number }[];
  flows: { from_zone: string; to_zone: string; count: number }[];
  heat_unit: string;
};
type ZoneComparison = {
  id: string;
  name: string;
  visits: number;
  pass_by: number;
  dwell_seconds: number | null;
  capture_rate: number | null;
};

const data = ref<Summary | null>(null);
const spatial = ref<Spatial>({ routes: [], heat: [], flows: [], heat_unit: "segundos-persona" });
const zoneFilter = ref("");
const genderFilter = ref("");
const dateFilter = ref("");
const hourFilter = ref("");
const slotFilter = ref("");
const loading = ref(false);
const error = ref("");
const zoneComparison = ref<ZoneComparison[]>([]);
const zoneComparisonLoading = ref(false);
const trend = ref<{ active: number[]; pass_by: number[]; dwell: number[]; capture: number[]; density: number[] }>({
  active: [],
  pass_by: [],
  dwell: [],
  capture: [],
  density: [],
});

const genderLabels: Record<string, string> = { HOMBRE: "Hombres", MUJER: "Mujeres", SIN_DETERMINAR: "Sin determinar" };

function baseParams() {
  const params = new URLSearchParams();
  if (dateFilter.value) {
    params.set("date", dateFilter.value);
    if (hourFilter.value !== "") params.set("hour", hourFilter.value);
    else if (slotFilter.value) params.set("time_slot", slotFilter.value);
  } else {
    params.set("minutes", "60");
  }
  if (genderFilter.value) params.set("gender", genderFilter.value);
  return params;
}
function query() {
  const params = baseParams();
  if (zoneFilter.value) params.set("zone_id", zoneFilter.value);
  return params.toString();
}
async function load() {
  loading.value = true;
  error.value = "";
  try {
    const suffix = query();
    const [summaryResponse, spatialResponse] = await Promise.all([
      fetch(`/api/v1/insights/summary?${suffix}`),
      fetch(`/api/v1/insights/spatial?${suffix}`),
    ]);
    if (!summaryResponse.ok || !spatialResponse.ok) throw Error("insights");
    data.value = await summaryResponse.json();
    spatial.value = await spatialResponse.json();
  } catch {
    error.value = "No se pudieron cargar los indicadores históricos.";
  } finally {
    loading.value = false;
  }
  loadTrend();
  loadZoneComparison();
}
function onHour() {
  if (hourFilter.value !== "") slotFilter.value = "";
  load();
}
function onSlot() {
  if (slotFilter.value) hourFilter.value = "";
  load();
}
const zones = computed(() => data.value?.zones ?? []);
const selectedZone = computed(() => zones.value.find((zone) => zone.id === zoneFilter.value));
// Qué mide cada tarjeta: todas las zonas o solo la elegida.
const scope = computed(() => (selectedZone.value ? `en ${selectedZone.value.name}` : "en todas las zonas"));
function dwell(seconds: number | null | undefined) {
  if (seconds == null) return "—";
  return seconds >= 60 ? `${(seconds / 60).toFixed(1)} min` : `${seconds.toFixed(0)} s`;
}
const max = computed(() => Math.max(1, ...(data.value?.series.map((value) => value.visits) ?? [])));

// Tendencia de las 5 métricas instantáneas: reutiliza GET /api/v1/replay/metrics
// (ya existe, acepta "at") muestreado en ~8 puntos del rango — sin endpoint nuevo.
async function loadTrend() {
  if (!data.value) return;
  const from = new Date(data.value.from).getTime();
  const to = new Date(data.value.to).getTime();
  const n = 8;
  const suffix = query();
  const samples = Array.from({ length: n }, (_, i) => new Date(from + ((to - from) * i) / (n - 1)).toISOString());
  const results = await Promise.all(
    samples.map((at) =>
      fetch(`/api/v1/replay/metrics?${suffix}&at=${encodeURIComponent(at)}`)
        .then((r) => (r.ok ? r.json() : null))
        .catch(() => null),
    ),
  );
  const valid = results.filter((r): r is Metrics => r != null);
  trend.value = {
    active: valid.map((m) => m.active),
    pass_by: valid.map((m) => m.pass_by),
    dwell: valid.map((m) => m.dwell_seconds ?? 0),
    capture: valid.map((m) => m.capture_rate ?? 0),
    density: valid.map((m) => m.density),
  };
}
// Comparación entre zonas: reutiliza GET /api/v1/insights/summary una vez por
// zona (zone_id=<id>), ignorando el filtro de tienda de arriba a propósito —
// el punto de este gráfico es comparar todas las zonas entre sí.
async function loadZoneComparison() {
  if (!data.value || !data.value.zones.length) {
    zoneComparison.value = [];
    return;
  }
  zoneComparisonLoading.value = true;
  const params = baseParams();
  const list = data.value.zones;
  try {
    const results = await Promise.all(
      list.map((zone) => {
        const p = new URLSearchParams(params);
        p.set("zone_id", zone.id);
        return fetch(`/api/v1/insights/summary?${p.toString()}`).then((r) => (r.ok ? r.json() : null));
      }),
    );
    zoneComparison.value = list
      .map((zone, i) => {
        const s = results[i] as Summary | null;
        return {
          id: zone.id,
          name: zone.name,
          visits: s?.metrics.visits ?? 0,
          pass_by: s?.metrics.pass_by ?? 0,
          dwell_seconds: s?.metrics.dwell_seconds ?? null,
          capture_rate: s?.metrics.capture_rate ?? null,
        };
      })
      .sort((a, b) => (b.dwell_seconds ?? 0) - (a.dwell_seconds ?? 0));
  } finally {
    zoneComparisonLoading.value = false;
  }
}

// --- sparklines: SVG a mano, sin libreria (mismo criterio que el resto del proyecto) ---
function sparklinePoints(values: number[], w = 72, h = 22, pad = 2) {
  if (values.length < 2) return "";
  const min = Math.min(...values);
  const max = Math.max(...values, min + 1e-6);
  const stepX = (w - pad * 2) / (values.length - 1);
  return values
    .map((v, i) => {
      const x = pad + i * stepX;
      const y = h - pad - ((v - min) / (max - min)) * (h - pad * 2);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
}

const zoneComparisonMaxDwell = computed(() => Math.max(1, ...zoneComparison.value.map((z) => z.dwell_seconds ?? 0)));

// --- exportación: PDF vía impresión del navegador, Excel como CSV ---
// Sin librerías nuevas (el proyecto no trae ninguna en el frontend): un
// stylesheet @media print para el PDF y un Blob de texto para el CSV.
const generatedAt = ref("");
const filterSummary = computed(() => {
  const parts: string[] = [];
  parts.push(selectedZone.value ? `Tienda: ${selectedZone.value.name}` : "Tienda: todas");
  parts.push(genderFilter.value ? `Género: ${genderLabels[genderFilter.value]}` : "Género: todos");
  if (dateFilter.value) {
    let when = `Fecha: ${dateFilter.value}`;
    if (hourFilter.value !== "") when += ` · ${String(hourFilter.value).padStart(2, "0")}:00`;
    else if (slotFilter.value) when += ` · franja ${slotFilter.value}`;
    parts.push(when);
  } else {
    parts.push("Rango: última hora con datos");
  }
  return parts.join("  ·  ");
});
function exportPDF() {
  generatedAt.value = new Date().toLocaleString("es-PE");
  window.print();
}
function csvCell(v: string | number): string {
  const s = String(v);
  return /[",\n;]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s;
}
function exportCSV() {
  if (!data.value) return;
  const rows: (string | number)[][] = [];
  rows.push(["Comportamiento de pasajeros · Jorge Chávez Nivel 3"]);
  rows.push([filterSummary.value]);
  rows.push([`Rango: ${new Date(data.value.from).toLocaleString("es-PE")} a ${new Date(data.value.to).toLocaleString("es-PE")}`]);
  rows.push([]);
  rows.push(["Indicador", "Valor"]);
  rows.push(["Activas", data.value.metrics.active]);
  rows.push(["Visitas", data.value.metrics.visits]);
  rows.push(["IDs anónimos", data.value.unique]);
  rows.push(["Pass-by", data.value.metrics.pass_by]);
  rows.push(["Dwell time (s)", data.value.metrics.dwell_seconds ?? ""]);
  rows.push(["Capture rate (%)", data.value.metrics.capture_rate ?? ""]);
  rows.push(["Expuestas", data.value.metrics.exposed]);
  rows.push(["Capturadas", data.value.metrics.captured]);
  rows.push(["Densidad (pers/m²)", data.value.metrics.density]);
  rows.push(["Visitas completas", data.value.complete]);
  rows.push(["Visitas censuradas", data.value.censored]);
  rows.push([]);
  rows.push(["Comparación entre zonas"]);
  rows.push(["Zona", "Visitas", "Pass-by", "Dwell time (s)", "Capture rate (%)"]);
  for (const z of zoneComparison.value) {
    rows.push([z.name, z.visits, z.pass_by, z.dwell_seconds ?? "", z.capture_rate ?? ""]);
  }
  rows.push([]);
  rows.push(["Flujo entre zonas"]);
  rows.push(["Origen", "Destino", "Personas"]);
  for (const f of spatial.value.flows) {
    rows.push([zones.value.find((z) => z.id === f.from_zone)?.name ?? f.from_zone, zones.value.find((z) => z.id === f.to_zone)?.name ?? f.to_zone, f.count]);
  }
  rows.push([]);
  rows.push(["Visitas por minuto"]);
  rows.push(["Hora", "Visitas"]);
  for (const p of data.value.series) {
    rows.push([new Date(p.at).toLocaleTimeString("es-PE", { hour: "2-digit", minute: "2-digit" }), p.visits]);
  }
  const csv = rows.map((row) => row.map(csvCell).join(",")).join("\r\n");
  // BOM: Excel detecta UTF-8 (tildes/ñ) solo si el archivo empieza con él.
  const blob = new Blob(["﻿" + csv], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `insights-comerciales-${new Date().toISOString().slice(0, 10)}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// --- flujo entre zonas: mini-sankey de dos columnas (origen -> destino) ---
const sankey = computed(() => {
  const flows = spatial.value.flows;
  const origins = [...new Set(flows.map((f) => f.from_zone))];
  const destinations = [...new Set(flows.map((f) => f.to_zone))];
  const maxCount = Math.max(1, ...flows.map((f) => f.count));
  const rowH = 34;
  const height = Math.max(origins.length, destinations.length) * rowH + 10;
  const nameOf = (id: string) => zones.value.find((z) => z.id === id)?.name ?? id;
  const yOf = (list: string[], id: string) => 10 + list.indexOf(id) * rowH + rowH / 2;
  return {
    height,
    origins: origins.map((id) => ({ id, name: nameOf(id), y: yOf(origins, id) })),
    destinations: destinations.map((id) => ({ id, name: nameOf(id), y: yOf(destinations, id) })),
    links: flows.map((f) => ({
      ...f,
      y1: yOf(origins, f.from_zone),
      y2: yOf(destinations, f.to_zone),
      width: 1.5 + (7 * f.count) / maxCount,
    })),
  };
});

onMounted(load);
</script>

<template>
  <section class="page-title">
    <div>
      <p class="eyebrow">LAP · NIVEL 3 / ANÁLISIS ESPACIAL</p>
      <h1>Comportamiento de pasajeros</h1>
      <p>Capas agregadas a partir de trayectorias históricas reales de PostgreSQL/PostGIS.</p>
    </div>
    <div class="replay-filters">
      <label>Tienda<select v-model="zoneFilter" @change="load">
          <option value="">Todas las zonas</option>
          <option v-for="zone in zones" :key="zone.id" :value="zone.id">
            {{ zone.name }}
          </option>
        </select></label
      ><label>Fecha<input v-model="dateFilter" type="date" @change="load" /></label>
      <label>Hora<select v-model="hourFilter" @change="onHour"><option value="">Todas</option><option v-for="hour in 24" :key="hour - 1" :value="hour - 1">{{ String(hour - 1).padStart(2, "0") }}:00</option></select></label>
      <label>Franja<select v-model="slotFilter" @change="onSlot" :disabled="!!hourFilter"><option value="">Todas</option><option value="night">00–06</option><option value="morning">06–12</option><option value="afternoon">12–18</option><option value="evening">18–24</option></select></label>
    </div>
    <div class="gender-slicer" role="group" aria-label="Filtro de género">
      <button type="button" :class="{ active: !genderFilter }" @click="genderFilter = ''; load()">Todos</button>
      <button
        v-for="(label, code) in genderLabels"
        :key="code"
        type="button"
        :class="{ active: genderFilter === code }"
        @click="genderFilter = code; load()"
      >{{ label }}</button>
    </div>
    <div class="export-actions">
      <button type="button" :disabled="!data" @click="exportPDF">⤓ Exportar PDF</button>
      <button type="button" :disabled="!data" @click="exportCSV">⤓ Exportar Excel (CSV)</button>
    </div>
  </section>
  <div v-if="data" class="print-header">
    <h1>LAP · Jorge Chávez · Comportamiento de pasajeros</h1>
    <p>{{ filterSummary }}</p>
    <p>Rango: {{ new Date(data.from).toLocaleString("es-PE") }} — {{ new Date(data.to).toLocaleString("es-PE") }}</p>
    <p>Generado {{ generatedAt }}</p>
  </div>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <p v-if="loading && !data" class="empty">Consultando histórico…</p>
  <div class="spatial-insights">
    <section class="spatial-maps">
      <article class="panel">
        <div class="panel-heading"><h2>Mapa de movimiento</h2><span class="pill">{{ spatial.routes.length }} recorridos</span></div>
        <LocalPlan horizontal :movements="spatial.routes" />
        <p class="chart-caption">Trayectorias reconstruidas por <code>global_id</code> y ordenadas por <code>timestamp</code>.</p>
      </article>
      <article class="panel">
        <div class="panel-heading"><h2>Mapa de calor</h2><span class="heat-legend">Menor ▰▰▰ Mayor permanencia</span></div>
        <LocalPlan horizontal :heat="spatial.heat" />
        <p class="chart-caption">{{ spatial.heat_unit }} acumulados por celda; cada intervalo se limita a 30 s para no extender huecos de observación.</p>
      </article>

      <article class="panel">
        <div class="panel-heading">
          <h2>Comparación entre zonas{{ zoneComparisonLoading ? " · actualizando…" : "" }}</h2>
          <span class="muted">Dwell time, visitas, pass-by y capture rate por tienda</span>
        </div>
        <p v-if="zoneComparison.length < zones.length" class="chart-caption zone-compare-note">
          Solo {{ zones.length }} zona{{ zones.length === 1 ? "" : "s" }} tiene{{ zones.length === 1 ? "" : "n" }} datos de recorridos hoy ({{ zones.map((z) => z.name).join(", ") || "ninguna" }}); las 12 tiendas de <code>locales</code>/<code>aero_zones</code> todavía no están conectadas a este tablero.
        </p>
        <ul v-if="zoneComparison.length" class="zone-compare-list">
          <li v-for="z in zoneComparison" :key="z.id">
            <div class="zone-compare-label">
              <b>{{ z.name }}</b>
              <span class="muted">{{ z.visits }} visitas · {{ z.pass_by }} pass-by · {{ z.capture_rate == null ? "—" : z.capture_rate.toFixed(0) + "% capture" }}</span>
            </div>
            <div class="zone-compare-bar-track">
              <div
                class="zone-compare-bar"
                :style="{ width: `${(100 * (z.dwell_seconds ?? 0)) / zoneComparisonMaxDwell}%` }"
              ></div>
              <span class="zone-compare-value">{{ dwell(z.dwell_seconds) }}</span>
            </div>
          </li>
        </ul>
        <p v-else class="empty">Sin datos de zonas para este filtro.</p>
      </article>
    </section>
    <aside v-if="data" class="insight-sidebar">
      <div class="metrics">
        <article class="panel metric">
          <p><span class="metric-icon">◍</span>ACTIVAS</p>
          <strong>{{ data.metrics.active }}</strong>
          <svg v-if="trend.active.length > 1" class="sparkline" viewBox="0 0 72 22"><polyline :points="sparklinePoints(trend.active)" /></svg>
          <small>{{ selectedZone ? "dentro de " + selectedZone.name : "en el cierre del rango" }}</small>
        </article>
        <article class="panel metric">
          <p><span class="metric-icon">⬡</span>VISITAS</p>
          <strong>{{ data.metrics.visits }}</strong>
          <svg v-if="data.series.length > 1" class="sparkline" viewBox="0 0 72 22"><polyline :points="sparklinePoints(data.series.map((p) => p.visits))" /></svg>
          <small>{{ data.unique }} IDs anónimos {{ scope }}</small>
        </article>
        <article class="panel metric">
          <p><span class="metric-icon">⇥</span>PASS-BY</p>
          <strong>{{ data.metrics.pass_by }}</strong>
          <svg v-if="trend.pass_by.length > 1" class="sparkline" viewBox="0 0 72 22"><polyline :points="sparklinePoints(trend.pass_by)" /></svg>
          <small>exposición registrada {{ scope }}</small>
        </article>
        <article class="panel metric">
          <p><span class="metric-icon">◷</span>DWELL TIME</p>
          <strong>{{ dwell(data.metrics.dwell_seconds) }}</strong>
          <svg v-if="trend.dwell.length > 1" class="sparkline" viewBox="0 0 72 22"><polyline :points="sparklinePoints(trend.dwell)" /></svg>
          <small>promedio por visita {{ scope }}</small>
        </article>
        <article class="panel metric">
          <p><span class="metric-icon">◎</span>CAPTURE RATE</p>
          <strong>{{ data.metrics.capture_rate == null ? "—" : data.metrics.capture_rate.toFixed(1) + "%" }}</strong>
          <svg v-if="trend.capture.length > 1" class="sparkline" viewBox="0 0 72 22"><polyline :points="sparklinePoints(trend.capture)" /></svg>
          <small>{{ data.metrics.captured }} de {{ data.metrics.exposed }} expuestas entraron {{ selectedZone ? "a " + selectedZone.name : "a una tienda" }}</small>
        </article>
        <article class="panel metric">
          <p><span class="metric-icon">▦</span>DENSIDAD</p>
          <strong>{{ data.metrics.density.toFixed(3) }}</strong>
          <svg v-if="trend.density.length > 1" class="sparkline" viewBox="0 0 72 22"><polyline :points="sparklinePoints(trend.density)" /></svg>
          <small>personas / m² {{ selectedZone ? "de " + selectedZone.name : "de zona" }}</small>
        </article>
      </div>
      <article class="panel">
        <div class="panel-heading"><h2>Visitas por minuto {{ selectedZone ? "· " + selectedZone.name : "" }}</h2><span class="muted">{{ data.complete }} completas · {{ data.censored }} censuradas</span></div>
        <div v-if="data.series.length" class="chart"><div v-for="point in data.series" :key="point.at" class="bar-col"><b>{{ point.visits }}</b><span class="bar" :style="{ height: `${18 + (70 * point.visits) / max}px` }"></span><small>{{ new Date(point.at).toLocaleTimeString("es-PE", { hour: "2-digit", minute: "2-digit" }) }}</small></div></div>
        <p v-else class="empty">Sin visitas en el filtro seleccionado.</p>
      </article>
      <article class="panel flow-panel">
        <div class="panel-heading"><h2>Flujo entre zonas</h2><span class="pill">{{ spatial.flows.length }}</span></div>
        <p v-if="!spatial.flows.length" class="empty">Sin transiciones registradas.</p>
        <svg v-else class="sankey" :viewBox="`0 0 220 ${sankey.height}`" :style="{ height: sankey.height + 'px' }">
          <path
            v-for="link in sankey.links"
            :key="link.from_zone + link.to_zone"
            :d="`M6,${link.y1} C90,${link.y1} 130,${link.y2} 214,${link.y2}`"
            fill="none"
            stroke="var(--blue-600)"
            :stroke-width="link.width"
            stroke-opacity=".55"
          />
          <g v-for="n in sankey.origins" :key="'o' + n.id">
            <circle cx="6" :cy="n.y" r="3" fill="var(--blue-600)" />
            <text x="11" :y="n.y" dy="3.2" class="sankey-label">{{ n.name }}</text>
          </g>
          <g v-for="n in sankey.destinations" :key="'d' + n.id">
            <circle cx="214" :cy="n.y" r="3" fill="var(--good)" />
            <text x="209" :y="n.y" dy="3.2" text-anchor="end" class="sankey-label">{{ n.name }}</text>
          </g>
          <text
            v-for="link in sankey.links"
            :key="'c' + link.from_zone + link.to_zone"
            x="110"
            :y="(link.y1 + link.y2) / 2"
            dy="-4"
            text-anchor="middle"
            class="sankey-count"
          >{{ link.count }}</text>
        </svg>
      </article>
    </aside>
  </div>
</template>
