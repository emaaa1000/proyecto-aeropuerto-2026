<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import PlanoEsan from "./PlanoEsan.vue";
import { api, TIPOS_ZONA, type Camara, type Config, type Punto } from "./api";

const config = ref<Config>();
const error = ref("");
const aviso = ref("");
const dibujando = ref(false);
const borrador = ref<Punto[]>([]);
const zonaActiva = ref<number | null>(null);
const nuevaZona = ref({ name: "", zone_type: "PASILLO", color: "#3d8bff" });
const edicion = ref<Record<string, { name: string; stream_uri: string; active: boolean }>>({});
const guardando = ref("");

const mapa = computed(() => config.value?.map?.mapa);
const camarasModelo = computed(() => (config.value?.cameras ?? []).filter((c) => c.camera_id !== "esan-movil"));
const informe = computed(() => {
  const inf = mapa.value?.informe ?? {};
  return Object.entries(inf)
    .filter(([clave]) => clave.includes("->"))
    .map(([par, v]) => ({ par, pares: v.pares ?? 0, residuo_mediano_m: v.residuo_mediano_m ?? 0, menos_de_1m: v.menos_de_1m ?? 0 }));
});

function pose(c: Camara) {
  return mapa.value?.camaras?.[c.camera_id];
}

async function cargar() {
  try {
    config.value = await api.config();
    edicion.value = Object.fromEntries(config.value.cameras.map((c) => [c.camera_id, { name: c.name, stream_uri: c.stream_uri, active: c.active }]));
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function guardarCamara(id: string) {
  guardando.value = id;
  error.value = aviso.value = "";
  try {
    await api.guardarCamara(id, edicion.value[id]);
    aviso.value = `Cámara ${id} guardada.`;
    await cargar();
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    guardando.value = "";
  }
}

function empezarZona() {
  dibujando.value = true;
  borrador.value = [];
  zonaActiva.value = null;
  aviso.value = "Haz clic en el plano para marcar los vértices; con 3 o más, pulsa Guardar zona.";
}

function cancelarZona() {
  dibujando.value = false;
  borrador.value = [];
  aviso.value = "";
}

async function guardarZona() {
  if (borrador.value.length < 3 || !nuevaZona.value.name.trim()) {
    error.value = "La zona necesita un nombre y al menos 3 vértices.";
    return;
  }
  error.value = "";
  try {
    const z = await api.guardarZona({ ...nuevaZona.value, name: nuevaZona.value.name.trim(), points: borrador.value });
    aviso.value = `Zona «${z.name}» guardada (${z.area_m2.toFixed(1)} m²).`;
    dibujando.value = false;
    borrador.value = [];
    nuevaZona.value.name = "";
    await cargar();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function borrarZona(id: number, nombre: string) {
  if (!confirm(`¿Eliminar la zona «${nombre}»?`)) return;
  try {
    await api.borrarZona(id);
    aviso.value = `Zona «${nombre}» eliminada.`;
    await cargar();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

onMounted(cargar);
</script>

<template>
  <section class="page-title">
    <div>
      <p class="eyebrow">ESAN · CONFIGURACIÓN DEL PLANO</p>
      <h1>Plano, cámaras y zonas</h1>
      <p>El plano y la calibración los calcula el modelo (Build Modelo, sección Mapa 2D); aquí se editan cámaras y zonas.</p>
    </div>
  </section>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <p v-if="aviso" class="success">{{ aviso }}</p>
  <p v-if="config && !mapa" class="empty panel">Todavía no hay plano de ESAN: ejecuta el Build con «Publicar en la base de datos».</p>

  <div v-if="config && mapa" class="config-esan">
    <section class="panel">
      <div class="panel-heading">
        <h2>Plano del piso · {{ (mapa.tam_px[0] / mapa.px_por_metro).toFixed(1) }} × {{ (mapa.tam_px[1] / mapa.px_por_metro).toFixed(1) }} m</h2>
        <span class="heading-meta">
          <button v-if="!dibujando" class="primary-button" type="button" @click="empezarZona">＋ Nueva zona</button>
          <template v-else>
            <button class="primary-button" type="button" :disabled="borrador.length < 3" @click="guardarZona">Guardar zona</button>
            <button type="button" :disabled="!borrador.length" @click="borrador.pop()">Deshacer vértice</button>
            <button type="button" @click="cancelarZona">Cancelar</button>
          </template>
        </span>
      </div>
      <div v-if="dibujando" class="form-zona">
        <label>Nombre<input v-model="nuevaZona.name" maxlength="100" placeholder="Ej. Ingreso principal" /></label>
        <label>Tipo<select v-model="nuevaZona.zone_type"><option v-for="(nombre, clave) in TIPOS_ZONA" :key="clave" :value="clave">{{ nombre }}</option></select></label>
        <label>Color<input v-model="nuevaZona.color" type="color" /></label>
        <span class="muted">{{ borrador.length }} vértices</span>
      </div>
      <PlanoEsan
        class="plano"
        :mapa="mapa"
        :zonas="config.zones"
        :borrador="borrador"
        :dibujando="dibujando"
        :zona-activa="zonaActiva"
        @punto="(p) => borrador.push(p)"
        @zona="(id) => (zonaActiva = id)"
      />
      <p class="chart-caption">Escala {{ mapa.px_por_metro }} px/m · cuadrícula de 1 m (líneas marcadas cada 5 m) · flechas: dirección de cada cámara.</p>
    </section>

    <aside class="columna">
      <section class="panel">
        <div class="panel-heading"><h2>Cámaras del modelo</h2><span class="pill">{{ camarasModelo.length }}</span></div>
        <article v-for="c in camarasModelo" :key="c.camera_id" class="camara">
          <header><b>{{ c.camera_id }}</b><span class="muted">{{ c.width_px }}×{{ c.height_px }} · {{ c.fps?.toFixed(0) }} fps</span></header>
          <dl v-if="pose(c)">
            <dt>Altura</dt><dd>{{ pose(c)!.altura_m.toFixed(2) }} m</dd>
            <dt>Focal</dt><dd>{{ pose(c)!.focal_px.toFixed(0) }} px</dd>
            <dt>Posición</dt><dd>({{ pose(c)!.posicion_m[0].toFixed(1) }}, {{ pose(c)!.posicion_m[1].toFixed(1) }}) m</dd>
            <dt>Desfase</dt><dd>{{ (mapa.desfases_s[c.camera_id] ?? 0).toFixed(2) }} s</dd>
          </dl>
          <label>Nombre<input v-model="edicion[c.camera_id].name" maxlength="80" /></label>
          <label>Fuente (RTSP / HTTP)<input v-model="edicion[c.camera_id].stream_uri" placeholder="rtsp://… o http://…" maxlength="500" /></label>
          <label class="check"><input v-model="edicion[c.camera_id].active" type="checkbox" /> Activa</label>
          <button type="button" :disabled="guardando === c.camera_id" @click="guardarCamara(c.camera_id)">Guardar cámara</button>
        </article>
      </section>

      <section class="panel">
        <div class="panel-heading"><h2>Zonas</h2><span class="pill">{{ config.zones.length }}</span></div>
        <ul class="zonas">
          <li v-for="z in config.zones" :key="z.zone_id" :class="{ activa: z.zone_id === zonaActiva }" @click="zonaActiva = z.zone_id">
            <span class="color" :style="{ background: z.color || '#3d8bff' }"></span>
            <span><b>{{ z.name }}</b><small class="muted">{{ TIPOS_ZONA[z.zone_type] ?? z.zone_type }} · {{ z.area_m2.toFixed(1) }} m²</small></span>
            <button class="danger-button icon-button" type="button" :aria-label="`Eliminar ${z.name}`" @click.stop="borrarZona(z.zone_id, z.name)">✕</button>
          </li>
          <li v-if="!config.zones.length" class="muted vacia">Sin zonas. Usa «Nueva zona» para medir visitas y permanencia por área en Insights.</li>
        </ul>
      </section>

      <section v-if="informe.length" class="panel">
        <div class="panel-heading"><h2>Calibración entre cámaras</h2></div>
        <table class="tabla">
          <thead><tr><th>Par</th><th>Pares</th><th>Residuo</th><th>&lt; 1 m</th></tr></thead>
          <tbody>
            <tr v-for="f in informe" :key="f.par"><td>{{ f.par }}</td><td>{{ f.pares }}</td><td>{{ f.residuo_mediano_m }} m</td><td>{{ Math.round((f.menos_de_1m ?? 0) * 100) }} %</td></tr>
          </tbody>
        </table>
      </section>
    </aside>
  </div>
</template>

<style scoped>
.config-esan {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 330px;
  gap: 14px;
  align-items: start;
}
.plano {
  margin: 10px;
  width: calc(100% - 20px);
}
.form-zona {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 14px 0;
}
.form-zona label,
.camara label {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 10.5px;
  font-weight: 650;
  color: var(--ink-soft);
}
.columna {
  display: grid;
  gap: 14px;
}
.camara {
  display: grid;
  gap: 7px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--glass-line);
}
.camara header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}
.camara dl {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 2px 10px;
  margin: 0;
  font-size: 11px;
}
.camara dt {
  color: var(--ink-faint);
}
.camara dd {
  margin: 0;
  font-variant-numeric: tabular-nums;
}
.camara .check {
  flex-direction: row;
  align-items: center;
  gap: 6px;
}
.zonas {
  list-style: none;
  margin: 0;
  padding: 8px 12px;
  display: grid;
  gap: 6px;
  font-size: 11.5px;
}
.zonas li {
  display: grid;
  grid-template-columns: 12px 1fr auto;
  align-items: center;
  gap: 8px;
  padding: 6px;
  border-radius: 8px;
  cursor: pointer;
}
.zonas li.vacia {
  display: block;
  cursor: default;
}
.zonas li.activa {
  background: var(--pill-bg);
}
.zonas small {
  display: block;
}
.color {
  width: 12px;
  height: 12px;
  border-radius: 3px;
}
.tabla {
  width: calc(100% - 24px);
  margin: 12px;
  border-collapse: collapse;
  font-size: 11px;
}
.tabla th,
.tabla td {
  padding: 6px 4px;
  text-align: left;
  border-bottom: 1px solid var(--glass-line);
}
@media (max-width: 1100px) {
  .config-esan {
    grid-template-columns: 1fr;
  }
}
</style>
