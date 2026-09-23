<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { RouterLink } from "vue-router";
import LocalPlan from "./LocalPlan.vue";
import type { MapObject } from "./mapObjects";
import {
  loadLocales,
  loadAeroZones,
  patchLocale,
  createAeroZone,
  patchAeroZone,
  type Locale,
  type AeroZone,
} from "./locales";

const zoneTypeLabels: Record<string, string> = {
  INTERIOR: "Interior del local",
  FRONTAGE: "Frente comercial",
  PASILLO: "Pasillo",
  ENTRADA: "Entrada",
  CHECKIN: "Check-in",
  SEGURIDAD: "Seguridad",
  PUERTA: "Puerta",
  COLA: "Cola",
  OTRO: "Otra",
};
const zoneTypeColor = "#e08a35";

const locales = ref<Locale[]>([]);
const zones = ref<AeroZone[]>([]);
const loading = ref(false);
const error = ref("");
const toasts = ref<{ id: number; kind: "success" | "error"; text: string }[]>([]);
let toastSeq = 0;
function toast(kind: "success" | "error", text: string) {
  const id = ++toastSeq;
  toasts.value.push({ id, kind, text });
  setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id);
  }, 3200);
}

// The zone currently being drawn or reshaped. `zoneId` is null until the
// first commit (>= 3 vertices) creates the row; from then on every change
// is a PATCH against that same id — there is no separate "Guardar" step.
const drawingLocalId = ref<number | null>(null);
const zoneId = ref<number | null>(null);
const zoneName = ref("");
const zoneType = ref("INTERIOR");
const vertices = ref<number[][]>([]);
const saving = ref(false);

const editing = computed(() => drawingLocalId.value !== null);
const handles = computed(() => vertices.value);

function zoneFor(localId: number): AeroZone | undefined {
  return zones.value.find((z) => z.local_id === localId);
}
function zoneAsObject(z: AeroZone): MapObject {
  return {
    id: String(z.zone_id),
    floor_id: z.floor_id,
    kind: "zone",
    name: z.name,
    category: z.zone_type,
    color: zoneTypeColor,
    source_ref: "",
    bearing: 0,
    geometry: z.geometry,
    coverage: null,
    revision: 0,
  };
}
// Every zone except the one being actively edited: that one is shown only as
// the live dashed vertex preview so it never renders twice.
const shownObjects = computed<MapObject[]>(() =>
  zones.value.filter((z) => z.zone_id !== zoneId.value).map(zoneAsObject),
);

async function refresh() {
  error.value = "";
  loading.value = true;
  try {
    [locales.value, zones.value] = await Promise.all([loadLocales(), loadAeroZones()]);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

function stopEditing() {
  drawingLocalId.value = null;
  zoneId.value = null;
  vertices.value = [];
  zoneName.value = "";
  zoneType.value = "INTERIOR";
}
function startNewZone(local: Locale) {
  stopEditing();
  drawingLocalId.value = local.local_id;
  zoneId.value = null;
  zoneName.value = "Zona - " + local.name;
  zoneType.value = "INTERIOR";
  vertices.value = [];
}
function startEditZone(local: Locale, zone: AeroZone) {
  stopEditing();
  drawingLocalId.value = local.local_id;
  zoneId.value = zone.zone_id;
  zoneName.value = zone.name;
  zoneType.value = zone.zone_type;
  vertices.value = zone.geometry.coordinates[0]!.slice(0, -1).map((p) => [...p]);
}
function selectZoneOnMap(id: string) {
  const zone = zones.value.find((z) => String(z.zone_id) === id);
  if (!zone) return;
  const local = locales.value.find((l) => l.local_id === zone.local_id);
  if (local) startEditZone(local, zone);
}

async function persistShape() {
  if (drawingLocalId.value === null || vertices.value.length < 3 || saving.value) return;
  const ring = [...vertices.value.map((p) => [...p]), [...vertices.value[0]!]];
  const geometry: GeoJSON.Polygon = { type: "Polygon", coordinates: [ring] };
  saving.value = true;
  try {
    if (zoneId.value === null) {
      const created = await createAeroZone({
        local_id: drawingLocalId.value,
        name: zoneName.value.trim() || "Zona sin nombre",
        zone_type: zoneType.value,
        geometry,
      });
      zoneId.value = created.zone_id;
      zones.value = [...zones.value, created];
      toast("success", "Zona creada y guardada.");
    } else {
      const saved = await patchAeroZone(zoneId.value, { geometry });
      zones.value = zones.value.map((z) => (z.zone_id === saved.zone_id ? saved : z));
      toast("success", "Zona actualizada.");
    }
  } catch (e) {
    toast("error", (e as Error).message);
  } finally {
    saving.value = false;
  }
}
async function saveZoneMeta() {
  if (zoneId.value === null) return;
  saving.value = true;
  try {
    const saved = await patchAeroZone(zoneId.value, {
      name: zoneName.value.trim() || "Zona sin nombre",
      zone_type: zoneType.value,
    });
    zones.value = zones.value.map((z) => (z.zone_id === saved.zone_id ? saved : z));
    toast("success", "Zona actualizada.");
  } catch (e) {
    toast("error", (e as Error).message);
  } finally {
    saving.value = false;
  }
}

function onPoint(p: number[]) {
  if (drawingLocalId.value === null || vertices.value.length >= 100) return;
  vertices.value.push(p);
  if (vertices.value.length >= 3) persistShape();
}
function onRemovePoint(index: number) {
  if (drawingLocalId.value === null) return;
  vertices.value.splice(index, 1);
  if (vertices.value.length >= 3) persistShape();
}
function onMoveHandle(i: number, p: number[]) {
  if (drawingLocalId.value === null) return;
  vertices.value[i] = p;
}
function onCommit() {
  persistShape();
}

const nameDrafts = ref<Record<number, string>>({});
const categoryDrafts = ref<Record<number, string>>({});
function nameFor(l: Locale) {
  return nameDrafts.value[l.local_id] ?? l.name;
}
function categoryFor(l: Locale) {
  return categoryDrafts.value[l.local_id] ?? l.category;
}
async function commitName(l: Locale) {
  const value = (nameDrafts.value[l.local_id] ?? l.name).trim();
  delete nameDrafts.value[l.local_id];
  if (!value || value === l.name) return;
  try {
    const saved = await patchLocale(l.local_id, { name: value });
    locales.value = locales.value.map((x) => (x.local_id === saved.local_id ? saved : x));
    toast("success", "Local actualizado.");
  } catch (e) {
    toast("error", (e as Error).message);
  }
}
async function commitCategory(l: Locale) {
  const value = (categoryDrafts.value[l.local_id] ?? l.category).trim();
  delete categoryDrafts.value[l.local_id];
  if (value === l.category) return;
  try {
    const saved = await patchLocale(l.local_id, { category: value });
    locales.value = locales.value.map((x) => (x.local_id === saved.local_id ? saved : x));
    toast("success", "Local actualizado.");
  } catch (e) {
    toast("error", (e as Error).message);
  }
}

function unload(e: BeforeUnloadEvent) {
  // Every change already persisted on commit — nothing to warn about.
  void e;
}
onMounted(() => {
  refresh();
  window.addEventListener("beforeunload", unload);
});
onUnmounted(() => window.removeEventListener("beforeunload", unload));
</script>
<template>
  <section class="page-title">
    <div>
      <p class="eyebrow">LAP · NIVEL 3 / CONFIGURACIÓN</p>
      <h1>Tiendas y locales comerciales</h1>
      <p>Edita el nombre y el polígono de cada local sobre el plano. Cada cambio se guarda solo.</p>
    </div>
  </section>
  <nav class="config-tabs">
    <RouterLink to="/configuracion">Cámaras y zonas</RouterLink>
    <RouterLink to="/configuracion/tiendas">Tiendas</RouterLink>
  </nav>
  <div class="editor-layout">
    <section class="panel editor-map-panel">
      <div class="panel-heading">
        <h2>Plano del aeropuerto · Nivel 3</h2>
        <span class="heading-meta"
          ><span class="pill">{{ zones.length }} zonas</span
          ><span class="pill">{{ editing ? "Edición activa" : "Vista fija" }}</span></span
        >
      </div>
      <div class="drawing-help" :class="{ drawing: editing }">
        <span>{{
          editing
            ? `Marca las esquinas del local (${vertices.length}). Cada punto, arrastre o borrado se guarda solo — sin botón Guardar.`
            : "Elige un local de la lista para dibujar o editar su zona, o haz clic directo sobre una zona pintada."
        }}</span>
        <button v-if="editing" @click="stopEditing">Listo</button>
      </div>
      <LocalPlan
        plan-space
        labels
        zoomable
        draw-color="#2868df"
        :objects="shownObjects"
        :draft="null"
        :vertices="vertices"
        :handles="handles"
        :editing="editing"
        :drawing="editing"
        @point="onPoint"
        @remove-point="onRemovePoint"
        @select="selectZoneOnMap"
        @move="onMoveHandle"
        @commit="onCommit"
      />
    </section>
    <aside class="panel properties-panel">
      <div class="panel-heading">
        <h2>Locales</h2>
        <span class="pill">{{ saving ? "Guardando…" : "Autoguardado" }}</span>
      </div>
      <div class="properties-body">
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <div v-if="editing" class="zone-editor">
          <label
            >Nombre de la zona<input
              v-model="zoneName"
              maxlength="100"
              @blur="zoneId !== null && saveZoneMeta()"
          /></label>
          <label
            >Tipo<select v-model="zoneType" @change="zoneId !== null && saveZoneMeta()">
              <option v-for="(l, k) in zoneTypeLabels" :key="k" :value="k">{{ l }}</option>
            </select></label
          >
          <p class="muted">
            {{
              zoneId === null
                ? "Marca al menos 3 esquinas en el plano; se crea sola al llegar a la tercera."
                : "Arrastra los puntos numerados para reubicar o redimensionar; suelta para guardar."
            }}
          </p>
        </div>
        <ul class="object-list">
          <li v-for="l in locales" :key="l.local_id">
            <div class="locale-row">
              <input
                class="locale-name-input"
                :value="nameFor(l)"
                @input="nameDrafts[l.local_id] = ($event.target as HTMLInputElement).value"
                @blur="commitName(l)"
                @keydown.enter="($event.target as HTMLInputElement).blur()"
                maxlength="100"
              />
              <input
                class="locale-category-input"
                :value="categoryFor(l)"
                @input="categoryDrafts[l.local_id] = ($event.target as HTMLInputElement).value"
                @blur="commitCategory(l)"
                @keydown.enter="($event.target as HTMLInputElement).blur()"
                maxlength="50"
                placeholder="Categoría"
              />
              <button
                v-if="zoneFor(l.local_id)"
                :class="{ 'primary-button': drawingLocalId === l.local_id }"
                @click="startEditZone(l, zoneFor(l.local_id)!)"
              >
                ⬡ Editar zona
              </button>
              <button v-else @click="startNewZone(l)">＋ Dibujar zona</button>
            </div>
          </li>
        </ul>
        <button @click="refresh" :disabled="loading">Actualizar lista</button>
      </div>
    </aside>
  </div>
  <div class="toast-stack" role="status" aria-live="polite">
    <div v-for="t in toasts" :key="t.id" class="toast" :class="t.kind">{{ t.text }}</div>
  </div>
</template>
<style scoped>
.locale-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
  width: 100%;
  padding: 0.4rem 0;
}
.locale-name-input {
  flex: 1 1 40%;
  min-width: 8rem;
  font-weight: 600;
}
.locale-category-input {
  flex: 1 1 30%;
  min-width: 6rem;
  font-size: 0.85rem;
  color: #64748b;
}
.zone-editor {
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 0.6rem;
  padding: 0.75rem;
  margin-bottom: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.toast-stack {
  position: fixed;
  right: 1rem;
  bottom: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  z-index: 40;
}
.toast {
  padding: 0.6rem 1rem;
  border-radius: 0.5rem;
  color: white;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.18);
  font-size: 0.9rem;
  animation: toast-in 0.15s ease-out;
}
.toast.success {
  background: #16a085;
}
.toast.error {
  background: #d64545;
}
@keyframes toast-in {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
