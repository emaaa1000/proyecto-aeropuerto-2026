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
const title = computed(() =>
  route.path === "/configuracion"
    ? "Cámaras y zonas del terminal"
    : route.path === "/configuracion/tiendas"
      ? "Tiendas y locales comerciales"
      : route.path === "/insights"
        ? "Análisis comercial"
        : route.path === "/esan"
          ? "ESAN - Análisis de Flujo"
          : "Control en vivo",
);

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
        <div>
          <b>Jorge Chávez</b><small>LIM · Lima, Perú · Nivel 3</small>
        </div>
      </div>
      <p class="nav-caption">ESPACIO DE TRABAJO</p>
      <nav aria-label="Navegación principal">
        <RouterLink to="/mapa"
          ><span>⌖</span
          ><span class="nav-label"><b class="nav-step">1.</b>En vivo</span
          ></RouterLink
        ><RouterLink to="/esan"
          ><span>📊</span
          ><span class="nav-label"><b class="nav-step">2.</b>ESAN</span
          ></RouterLink
        ><RouterLink to="/insights"
          ><span>▥</span
          ><span class="nav-label"><b class="nav-step">3.</b>Insights</span
          ></RouterLink
        ><RouterLink to="/configuracion"
          ><span>◇</span
          ><span class="nav-label"
            ><b class="nav-step">4.</b>Configuración</span
          ></RouterLink
        >
      </nav>
      <div class="sidebar-bottom">
        <span class="demo-indicator"></span>
        <div>
          <b>Entorno de demostración</b
          ><small>Personas y recorridos simulados</small>
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
      <main><RouterView /></main>
      <footer>
        <span
          >Lima Airport Partners · Aeropuerto Internacional Jorge Chávez</span
        ><span>Plano local · Demostración sin conexión a CCTV</span>
      </footer>
    </div>
  </div>
</template>
