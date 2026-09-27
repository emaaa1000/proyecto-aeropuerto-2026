<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { wsUrl } from "../../../core/http";
import { segundos } from "../../../shared/format";
import { api, type EstadoServicio, type Telefono } from "../api";
import VistaTelefono from "../components/VistaTelefono.vue";

const telefonos = ref<Telefono[]>([]);
const nuevo = ref({ nombre: "", url: "" });
const error = ref("");
const aviso = ref("");
const servicio = ref<EstadoServicio>();
const ultimoEstado = ref(0);
const ahora = ref(Date.now());
let socket: WebSocket | undefined;
let reintento: ReturnType<typeof setTimeout> | undefined;
let sondeo: ReturnType<typeof setInterval> | undefined;
let reloj: ReturnType<typeof setInterval> | undefined;
let activo = true;

const conectado = computed(() => ahora.value - ultimoEstado.value < 6000 && servicio.value?.estado !== "detenido");
const textoEstado = computed(() => {
  if (!conectado.value) return { clase: "warn", texto: "El servicio del modelo no está corriendo (paso 3)." };
  if (servicio.value?.estado === "sin_telefonos") return { clase: "warn", texto: "Servicio listo: agrega un teléfono (paso 2)." };
  const procesando = Object.values(servicio.value?.telefonos ?? {}).filter((t) => t.estado === "procesando").length;
  return { clase: procesando ? "good" : "warn", texto: `Procesando ${procesando} de ${telefonos.value.length} teléfonos en ${servicio.value?.dispositivo ?? ""}` };
});
const generos = computed(() => {
  const g = servicio.value?.genero ?? {};
  return { hombres: g["Hombre"] ?? 0, mujeres: g["Mujer"] ?? 0, otros: g["Sin determinar"] ?? 0 };
});

async function cargar() {
  try {
    telefonos.value = await api.telefonos();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function agregar() {
  error.value = aviso.value = "";
  try {
    const t = await api.agregar({ nombre: nuevo.value.nombre.trim(), url: nuevo.value.url.trim() });
    aviso.value = `${t.nombre} agregado: el servicio del modelo lo toma en unos segundos.`;
    nuevo.value = { nombre: "", url: "" };
    await cargar();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function quitar(t: Telefono) {
  error.value = aviso.value = "";
  try {
    await api.quitar(t.id);
    aviso.value = `${t.nombre} quitado.`;
    await cargar();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

function escucharServicio() {
  socket = new WebSocket(wsUrl("/api/v1/cameras/telefonos/detections/watch"));
  socket.addEventListener("message", (ev) => {
    try {
      servicio.value = JSON.parse(ev.data as string) as EstadoServicio;
      ultimoEstado.value = Date.now();
    } catch {
      return;
    }
  });
  socket.addEventListener("close", () => {
    if (activo) reintento = setTimeout(escucharServicio, 2000);
  });
  socket.addEventListener("error", () => socket?.close());
}

onMounted(() => {
  cargar();
  escucharServicio();
  sondeo = setInterval(cargar, 4000);
  reloj = setInterval(() => (ahora.value = Date.now()), 1000);
});

onUnmounted(() => {
  activo = false;
  clearTimeout(reintento);
  clearInterval(sondeo);
  clearInterval(reloj);
  socket?.close();
});
</script>

<template>
  <section class="page-title">
    <div>
      <p class="eyebrow">MODELO FINAL · CÁMARAS DE TELÉFONO</p>
      <h1>Tracking en vivo con tus teléfonos</h1>
      <p>
        Cada teléfono con la app es una cámara IP; el modelo final (YOLO26m + tracker + Re-ID + género) corre en la GPU de esta laptop sobre todos a la
        vez, con un mismo ID por persona entre teléfonos. <b>No se guarda nada</b>: todo vive en memoria y se borra al cerrar el servicio.
      </p>
    </div>
    <span :class="textoEstado.clase" class="estado-servicio">● {{ textoEstado.texto }}</span>
  </section>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <p v-if="aviso" class="success">{{ aviso }}</p>

  <div v-if="conectado && servicio?.estado === 'procesando'" class="resumen panel">
    <div><b>{{ servicio.personas_total ?? 0 }}</b><small>personas únicas</small></div>
    <div><b>{{ servicio.multitelefono ?? 0 }}</b><small>vistas en varios teléfonos</small></div>
    <div><b>{{ generos.hombres }}</b><small>hombres</small></div>
    <div><b>{{ generos.mujeres }}</b><small>mujeres</small></div>
    <div><b>{{ generos.otros }}</b><small>sin determinar</small></div>
    <div><b>{{ segundos(servicio.segundos) }}</b><small>en memoria</small></div>
  </div>

  <div class="telefono">
    <div class="vistas" :class="{ varias: telefonos.length > 1 }">
      <VistaTelefono
        v-for="t in telefonos"
        :id="t.id"
        :key="t.id"
        :nombre="t.nombre"
        :estado="conectado ? servicio?.telefonos[t.id] : undefined"
        @quitar="quitar(t)"
      />
      <section v-if="!telefonos.length" class="panel vacio">
        <p>Sin teléfonos todavía. Sigue los pasos de «Cómo conectarlos».</p>
      </section>
    </div>

    <aside class="panel pasos">
      <div class="panel-heading"><h2>Cómo conectarlos</h2></div>
      <ol>
        <li>
          <b>Instala la app en cada teléfono Android.</b>
          <a class="button" href="/descargas/CamaraESAN.apk" download>⬇ CamaraESAN.apk</a>
          <small class="muted">Ábrela, concede la cámara y pulsa <b>Transmitir</b>. Teléfonos y laptop en la misma red Wi-Fi.</small>
        </li>
        <li>
          <b>Agrega cada teléfono con la URL que muestra su app.</b>
          <input v-model="nuevo.nombre" maxlength="40" placeholder="Nombre (opcional), ej. Entrada" />
          <input v-model="nuevo.url" placeholder="http://192.168.1.50:8080/video?token=…" @keyup.enter="agregar" />
          <button class="primary-button" type="button" :disabled="!nuevo.url.trim()" @click="agregar">＋ Agregar teléfono</button>
          <small class="muted">Hasta 8 teléfonos. La lista vive en la memoria del backend: si se reinicia, vuelve a agregarlos.</small>
        </li>
        <li>
          <b>Inicia el modelo en la laptop (GPU).</b>
          <code>python "Modelo/Test Modelo/camara_telefono.py"</code>
          <small class="muted"
            >Sin teléfono a mano: <code>python "Modelo/Test Modelo/simular_telefono.py"</code> y la URL
            <code>http://127.0.0.1:8090/video?token=prueba</code>.</small
          >
        </li>
        <li><b>Mira el tracking aquí.</b> Al cerrar el servicio (Ctrl+C) se descarta todo lo procesado.</li>
      </ol>
    </aside>
  </div>
</template>

<style scoped>
.estado-servicio {
  font-size: 11.5px;
}
.resumen {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 8px;
  padding: 10px;
  margin-bottom: 14px;
}
.resumen div {
  display: flex;
  flex-direction: column;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--pill-bg);
}
.resumen b {
  font-size: 18px;
  font-variant-numeric: tabular-nums;
}
.resumen small {
  font-size: 10px;
  color: var(--ink-soft);
}
.telefono {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 14px;
  align-items: start;
}
.vistas {
  display: grid;
  gap: 14px;
}
.vistas.varias {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.vacio p {
  display: grid;
  place-items: center;
  min-height: 240px;
  margin: 0;
  font-size: 12px;
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
  .telefono,
  .vistas.varias {
    grid-template-columns: 1fr;
  }
  .resumen {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
