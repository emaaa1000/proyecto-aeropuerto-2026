import { createApp } from "vue";
import { createRouter, createWebHistory } from "vue-router";
import App from "./App.vue";
import "./style.css";
const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", redirect: "/mapa" },
    { path: "/login", component: () => import("./LoginPage.vue"), meta: { public: true } },
    { path: "/mapa", component: () => import("./MapPage.vue") },
    { path: "/mapa/esan", component: () => import("./esan/EsanEnVivo.vue") },
    { path: "/configuracion", component: () => import("./EditorPage.vue") },
    { path: "/configuracion/tiendas", component: () => import("./TiendasPage.vue") },
    { path: "/configuracion/esan", component: () => import("./esan/EsanConfiguracion.vue") },
    { path: "/insights", component: () => import("./InsightsPage.vue") },
    { path: "/insights/esan", component: () => import("./esan/EsanInsights.vue") },
    { path: "/telefono", component: () => import("./TelefonoPage.vue") },
    { path: "/esan", redirect: "/mapa/esan" },
    { path: "/:pathMatch(.*)*", redirect: "/mapa" },
  ],
});

router.beforeEach((to) => {
  const isAuthenticated = Boolean(localStorage.getItem("lap-session"));
  if (to.meta.public && isAuthenticated) return "/mapa";
  if (!to.meta.public && !isAuthenticated) return "/login";
  return true;
});

createApp(App).use(router).mount("#app");
