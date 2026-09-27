<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute } from "vue-router";
import { useRouter } from "vue-router";
const route = useRoute();
const router = useRouter();

const theme = ref<"light" | "dark">(
  (document.documentElement.getAttribute("data-theme") as "light" | "dark") || "light",
);
function toggleTheme() {
  theme.value = theme.value === "dark" ? "light" : "dark";
  document.documentElement.setAttribute("data-theme", theme.value);
  try {
    localStorage.setItem("lap-theme", theme.value);
  } catch {
    /* localStorage puede estar bloqueado (modo privado); el tema no persiste. */
  }
}
const esan = computed(() => route.path.endsWith("/esan") || route.path === "/telefono");
const seccion = computed(() => ["/mapa", "/insights", "/configuracion"].find((s) => route.path.startsWith(s)) ?? "");
const title = computed(() => {
  if (route.path === "/telefono") return "Cámara del teléfono · modelo final";
  const sitio = esan.value ? "ESAN" : "LAP";
  if (seccion.value === "/configuracion")
    return esan.value ? "ESAN · Plano, cámaras y zonas" : route.path === "/configuracion/tiendas" ? "Tiendas y locales comerciales" : "Cámaras y zonas del terminal";
  if (seccion.value === "/insights") return `${sitio} · Insights`;
  return `${sitio} · En vivo`;
});

function logout() {
  localStorage.removeItem("lap-session");
  router.replace("/login");
}
</script>
<template>
  <div v-if="route.path === '/login'"><RouterView /></div>
  <div v-else class="app-shell">
    <aside class="sidebar">
      <a class="brand" href="/configuracion"
        ><span class="brand-icon">✈</span
        ><span>LAP<small>LIMA AIRPORT PARTNERS</small></span></a
      >
      <div class="airport-switch">
        <span class="airport-symbol">⌖</span>
        <div v-if="esan"><b>ESAN</b><small>Lima, Perú · 3 cámaras · plano en metros</small></div>
        <div v-else><b>Jorge Chávez</b><small>LIM · Lima, Perú · Nivel 3</small></div>
      </div>
      <p class="nav-caption">ESPACIO DE TRABAJO</p>
      <nav aria-label="Navegación principal">
        <RouterLink :to="esan ? '/mapa/esan' : '/mapa'" :class="{ 'router-link-active': seccion === '/mapa' }"
          ><span>⌖</span
          ><span class="nav-label"><b class="nav-step">1.</b>En vivo</span
          ></RouterLink
        ><RouterLink :to="esan ? '/insights/esan' : '/insights'" :class="{ 'router-link-active': seccion === '/insights' }"
          ><span>▥</span
          ><span class="nav-label"><b class="nav-step">2.</b>Insights</span
          ></RouterLink
        ><RouterLink :to="esan ? '/configuracion/esan' : '/configuracion'" :class="{ 'router-link-active': seccion === '/configuracion' }"
          ><span>◇</span
          ><span class="nav-label"
            ><b class="nav-step">3.</b>Configuración</span
          ></RouterLink
        ><RouterLink to="/telefono"
          ><span>📱</span
          ><span class="nav-label"><b class="nav-step">4.</b>Cámara del teléfono</span
          ></RouterLink
        >
      </nav>
      <div class="sidebar-bottom">
        <span class="demo-indicator"></span>
        <div>
          <b>{{ esan ? "Modelo LAP01" : "Entorno de demostración" }}</b
          ><small>{{ esan ? "Resultados reales guardados en la base" : "Personas y recorridos simulados" }}</small>
        </div>
      </div>
    </aside>
    <div class="workspace">
      <header class="workspace-header">
        <div><span class="breadcrumb">LAP /</span> {{ title }}</div>
        <div class="workspace-meta">
          <span class="live-dot"></span> Demo local
          <button
            class="theme-toggle"
            type="button"
            @click="toggleTheme"
            :aria-label="theme === 'dark' ? 'Cambiar a modo claro' : 'Cambiar a modo oscuro'"
            :title="theme === 'dark' ? 'Modo claro' : 'Modo oscuro'"
          >{{ theme === "dark" ? "☀" : "☾" }}</button>
          <span class="avatar" title="Sesión LAP">LAP</span>
          <button class="logout-button" type="button" @click="logout">Salir</button>
        </div>
      </header>
      <main>
        <nav v-if="seccion" class="config-tabs sitio-tabs" aria-label="Sitio">
          <RouterLink :to="seccion" :class="{ activo: !esan }">LAP · Jorge Chávez</RouterLink>
          <RouterLink :to="seccion + '/esan'" :class="{ activo: esan }">ESAN</RouterLink>
        </nav>
        <RouterView />
      </main>
      <footer>
        <span v-if="esan || route.path === '/telefono'">ESAN · modelo LAP01 (YOLO26m + tracking + Re-ID + mapa 2D)</span
        ><span v-else>Lima Airport Partners · Aeropuerto Internacional Jorge Chávez</span
        ><span>{{ esan || route.path === '/telefono' ? "Datos del modelo guardados en PostgreSQL/PostGIS" : "Plano local · Demostración sin conexión a CCTV" }}</span>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.sitio-tabs a.activo {
  color: white;
  background: var(--blue-600);
}
.sitio-tabs a.router-link-exact-active:not(.activo) {
  color: var(--ink-soft);
  background: rgba(191, 230, 255, 0.35);
}
</style>
