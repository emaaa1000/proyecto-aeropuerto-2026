<script setup lang="ts">
import { computed, ref } from "vue";
import type { Mapa, Punto, Zona } from "./api";

export type PersonaPlano = { id: number; x: number; y: number; color: string; etiqueta: string; rastro?: Punto[] };

const props = withDefaults(
  defineProps<{
    mapa: Mapa;
    zonas?: Zona[];
    personas?: PersonaPlano[];
    calor?: [number, number, number][];
    borrador?: Punto[];
    dibujando?: boolean;
    zonaActiva?: number | null;
    mostrarCamaras?: boolean;
  }>(),
  { zonas: () => [], personas: () => [], calor: () => [], borrador: () => [], dibujando: false, zonaActiva: null, mostrarCamaras: true },
);
const emit = defineEmits<{ punto: [p: Punto]; zona: [id: number] }>();
const svg = ref<SVGSVGElement>();

const k = computed(() => props.mapa.px_por_metro);
const ancho = computed(() => props.mapa.tam_px[0]);
const alto = computed(() => props.mapa.tam_px[1]);
const px = (x: number) => (x - props.mapa.origen_m[0]) * k.value;
const py = (y: number) => (props.mapa.origen_m[1] - y) * k.value;
const puntos = (ps: Punto[]) => ps.map(([x, y]) => `${px(x).toFixed(1)},${py(y).toFixed(1)}`).join(" ");

const lineas = computed(() => {
  const [x0, y1] = props.mapa.origen_m;
  const ancho_m = ancho.value / k.value;
  const alto_m = alto.value / k.value;
  const verticales = [];
  for (let x = Math.ceil(x0); x <= x0 + ancho_m; x++) verticales.push({ v: px(x), fuerte: x % 5 === 0 });
  const horizontales = [];
  for (let y = Math.floor(y1); y >= y1 - alto_m; y--) horizontales.push({ v: py(y), fuerte: y % 5 === 0 });
  return { verticales, horizontales };
});

const camaras = computed(() =>
  Object.entries(props.mapa.camaras ?? {}).map(([id, c]) => {
    const x = Math.min(Math.max(px(c.posicion_m[0]), 14), ancho.value - 110);
    const y = Math.min(Math.max(py(c.posicion_m[1]), 14), alto.value - 14);
    const largo = 3 * k.value;
    return { id, x, y, x2: x + c.direccion[0] * largo, y2: y - c.direccion[1] * largo, altura: c.altura_m };
  }),
);

const maxCalor = computed(() => Math.max(1, ...props.calor.map((c) => c[2])));
const celdas = computed(() =>
  props.calor.map(([cx, cy, n]) => ({
    x: px(cx),
    y: py(cy + 1),
    lado: k.value,
    color: `rgba(255, ${Math.round(200 - 170 * (n / maxCalor.value))}, 40, ${(0.18 + 0.7 * (n / maxCalor.value)).toFixed(2)})`,
    n,
  })),
);

const centro = (ps: Punto[]): Punto => [ps.reduce((a, p) => a + p[0], 0) / ps.length, ps.reduce((a, p) => a + p[1], 0) / ps.length];

function clic(ev: MouseEvent) {
  if (!props.dibujando || !svg.value) return;
  const matriz = svg.value.getScreenCTM();
  if (!matriz) return;
  const p = new DOMPoint(ev.clientX, ev.clientY).matrixTransform(matriz.inverse());
  emit("punto", [+(p.x / k.value + props.mapa.origen_m[0]).toFixed(2), +(props.mapa.origen_m[1] - p.y / k.value).toFixed(2)]);
}
</script>

<template>
  <svg
    ref="svg"
    class="plano-esan"
    :class="{ dibujando }"
    :viewBox="`0 0 ${ancho} ${alto}`"
    preserveAspectRatio="xMidYMid meet"
    role="img"
    aria-label="Plano del piso ESAN en metros"
    @click="clic"
  >
    <rect :width="ancho" :height="alto" class="fondo" />
    <polygon v-if="mapa.piso_L_m?.length" :points="puntos(mapa.piso_L_m)" class="piso" />
    <line v-for="l in lineas.verticales" :key="'v' + l.v" :x1="l.v" :x2="l.v" y1="0" :y2="alto" :class="l.fuerte ? 'rejilla-5' : 'rejilla'" />
    <line v-for="l in lineas.horizontales" :key="'h' + l.v" :y1="l.v" :y2="l.v" x1="0" :x2="ancho" :class="l.fuerte ? 'rejilla-5' : 'rejilla'" />
    <rect v-for="(c, i) in celdas" :key="'c' + i" :x="c.x" :y="c.y" :width="c.lado" :height="c.lado" :fill="c.color"><title>{{ c.n }} segundos-persona</title></rect>
    <g v-for="z in zonas" :key="'z' + z.zone_id" class="zona" :class="{ activa: z.zone_id === zonaActiva }" @click.stop="emit('zona', z.zone_id)">
      <polygon :points="puntos(z.points)" :style="{ fill: (z.color || '#3d8bff') + '33', stroke: z.color || '#3d8bff' }" />
      <text :x="px(centro(z.points)[0])" :y="py(centro(z.points)[1])" class="zona-nombre">{{ z.name }}</text>
    </g>
    <g v-if="borrador.length">
      <polyline :points="puntos(borrador)" class="borrador" />
      <circle v-for="(p, i) in borrador" :key="'b' + i" :cx="px(p[0])" :cy="py(p[1])" r="4" class="vertice" />
    </g>
    <g v-if="mostrarCamaras">
      <g v-for="c in camaras" :key="c.id" class="camara">
        <line :x1="c.x" :y1="c.y" :x2="c.x2" :y2="c.y2" marker-end="url(#flecha)" />
        <circle :cx="c.x" :cy="c.y" r="7" />
        <text :x="c.x + 11" :y="c.y + 4">{{ c.id }} · {{ c.altura.toFixed(1) }} m</text>
      </g>
    </g>
    <g v-for="p in personas" :key="'p' + p.id">
      <polyline v-if="p.rastro && p.rastro.length > 1" :points="puntos(p.rastro)" class="rastro" :style="{ stroke: p.color }" />
      <circle :cx="px(p.x)" :cy="py(p.y)" r="9" :fill="p.color" class="persona" />
      <text :x="px(p.x) + 12" :y="py(p.y) - 8" class="persona-etiqueta" :style="{ fill: p.color }">{{ p.etiqueta }}</text>
    </g>
    <g class="escala" :transform="`translate(14, ${alto - 18})`">
      <rect :width="5 * k" height="5" />
      <text :x="5 * k + 6" y="6">5 m</text>
    </g>
    <defs>
      <marker id="flecha" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
        <path d="M 0 0 L 10 5 L 0 10 z" class="punta" />
      </marker>
    </defs>
  </svg>
</template>

<style scoped>
.plano-esan {
  display: block;
  width: 100%;
  height: auto;
  border-radius: var(--radius-s);
  user-select: none;
}
.plano-esan.dibujando {
  cursor: crosshair;
}
.fondo {
  fill: var(--glass-strong);
}
.piso {
  fill: rgba(61, 139, 255, 0.07);
  stroke: var(--ink-faint);
  stroke-width: 2;
}
.rejilla {
  stroke: rgba(139, 164, 191, 0.18);
  stroke-width: 1;
}
.rejilla-5 {
  stroke: rgba(139, 164, 191, 0.42);
  stroke-width: 1;
}
.zona polygon {
  stroke-width: 2;
  cursor: pointer;
}
.zona.activa polygon {
  stroke-width: 4;
}
.zona-nombre {
  font-size: 13px;
  font-weight: 650;
  fill: var(--ink);
  text-anchor: middle;
  pointer-events: none;
}
.borrador {
  fill: rgba(61, 139, 255, 0.15);
  stroke: var(--blue-600);
  stroke-width: 2;
  stroke-dasharray: 6 4;
}
.vertice {
  fill: var(--blue-600);
}
.camara line {
  stroke: var(--ink-soft);
  stroke-width: 2;
}
.camara circle {
  fill: var(--ink);
}
.camara text,
.escala text {
  font-size: 12px;
  font-weight: 600;
  fill: var(--ink-soft);
}
.punta {
  fill: var(--ink-soft);
}
.escala rect {
  fill: var(--ink-soft);
}
.rastro {
  fill: none;
  stroke-width: 3;
  stroke-linecap: round;
  stroke-linejoin: round;
  opacity: 0.55;
}
.persona {
  stroke: #fff;
  stroke-width: 2;
}
.persona-etiqueta {
  font-size: 13px;
  font-weight: 700;
  paint-order: stroke;
  stroke: var(--glass-strong);
  stroke-width: 3px;
}
</style>
