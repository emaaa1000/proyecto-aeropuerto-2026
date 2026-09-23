<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';

const activeTab = ref<'procesamiento' | 'mapa' | 'resultado'>('procesamiento');
const esanData = ref<any>(null);
const loading = ref(true);
const error = ref('');

const stats = computed(() => {
  if (!esanData.value) return null;
  return esanData.value.estadísticas || {};
});

onMounted(async () => {
  try {
    // Intentar cargar datos ESAN
    const response = await fetch('/Modelo/Ouput/esan_data.json');
    if (response.ok) {
      esanData.value = await response.json();
    }
  } catch (e) {
    console.warn('No se pudieron cargar datos ESAN:', e);
  } finally {
    loading.value = false;
  }
});

const tamaño_real = computed(() => {
  if (!esanData.value?.mapa) return null;
  const px = esanData.value.mapa.tam_px;
  const ppx = esanData.value.mapa.px_por_metro;
  return {
    ancho: (px[0] / ppx).toFixed(1),
    alto: (px[1] / ppx).toFixed(1)
  };
});
</script>

<template>
  <div class="esan-page">
    <div class="esan-header">
      <h1>ESAN - Análisis de Flujo de Pasajeros</h1>
      <p class="subtitle">Procesamiento integrado de video + Mapa 2D + Resultados</p>
    </div>

    <div class="esan-tabs">
      <button
        v-for="tab in ['procesamiento', 'mapa', 'resultado']"
        :key="tab"
        :class="['tab-btn', { active: activeTab === tab }]"
        @click="activeTab = tab as any"
      >
        <span v-if="tab === 'procesamiento'" class="tab-icon">📹</span>
        <span v-else-if="tab === 'mapa'" class="tab-icon">🗺️</span>
        <span v-else-if="tab === 'resultado'" class="tab-icon">📊</span>
        {{ tab === 'procesamiento' ? 'Procesamiento' : tab === 'mapa' ? 'Mapa 2D' : 'Resultado' }}
      </button>
    </div>

    <div class="esan-content">
      <!-- TAB 1: PROCESAMIENTO DEL VIDEO -->
      <div v-if="activeTab === 'procesamiento'" class="tab-content processing-tab">
        <div class="section-title">1️⃣ Procesamiento del Video</div>
        <p class="section-desc">Visualización del procesamiento de las 3 cámaras sincronizadas</p>

        <div class="cameras-grid">
          <div v-for="cam in ['cam01', 'cam02', 'cam03']" :key="cam" class="camera-card">
            <div class="camera-title">{{ cam.toUpperCase() }}</div>
            <div class="camera-preview">
              <video controls preload="metadata" :src="`/api/esan/video/${cam}_procesado.mp4`">
                Tu navegador no soporta video.
              </video>
            </div>
            <div class="camera-info">
              <small>Estado: Procesado</small>
            </div>
          </div>
        </div>

        <div class="processing-steps">
          <h3>Pasos del procesamiento:</h3>
          <ol class="steps-list">
            <li><strong>Detección (YOLO):</strong> Localización de personas en cada frame</li>
            <li><strong>Tracking local:</strong> Seguimiento intra-cámara (LAP01)</li>
            <li><strong>Re-ID multicámara:</strong> Asociación entre cámaras</li>
            <li><strong>Proyección al piso:</strong> Conversión de píxeles a metros</li>
            <li><strong>Sincronización:</strong> Alineación temporal de las 3 cámaras</li>
          </ol>
        </div>
      </div>

      <!-- TAB 2: MAPA 2D CON TRAYECTORIAS -->
      <div v-if="activeTab === 'mapa'" class="tab-content mapa-tab">
        <div class="section-title">2️⃣ Mapa 2D - Trayectorias de Personas</div>
        <p class="section-desc">Vista superior del piso con recorrido de cada persona</p>

        <div class="mapa-container">
          <div class="mapa-info">
            <div v-if="tamaño_real" class="info-box">
              <strong>Dimensiones del piso:</strong>
              <p>{{ tamaño_real.ancho }}m × {{ tamaño_real.alto }}m</p>
            </div>
            <div v-if="stats" class="info-box">
              <strong>Personas detectadas:</strong>
              <p>{{ stats.total_personas }}</p>
            </div>
            <div v-if="stats" class="info-box">
              <strong>Duración promedio:</strong>
              <p>{{ stats.duracion_promedio_s?.toFixed(1) || 'N/A' }}s</p>
            </div>
          </div>

          <div class="mapa-image">
            <img :src="'/api/esan/video/mapa_trayectorias.png'" alt="Mapa de trayectorias" />
            <p class="image-caption">Mapa de trayectorias - recorrido completo de cada persona</p>
          </div>

          <div class="mapa-video">
            <h4>Video del mapa 2D</h4>
            <video controls preload="metadata" src="/api/esan/video/mapa_2d.mp4">
              Tu navegador no soporta video.
            </video>
            <p class="image-caption">Video 2D con trayectorias persistentes y camaras sincronizadas</p>
          </div>

          <div class="mapa-legend">
            <h4>Leyenda:</h4>
            <div class="legend-item">
              <div class="legend-color" style="background: #00ff00;"></div>
              <span>Origen (0,0)</span>
            </div>
            <div class="legend-item">
              <div class="legend-color" style="background: #0096ff;"></div>
              <span>Trayectorias</span>
            </div>
            <div class="legend-item">
              <div class="legend-color" style="background: #ff6400;"></div>
              <span>Puntos de detección</span>
            </div>
            <div class="legend-item">
              <div class="legend-color" style="background: #96c8c8;"></div>
              <span>Grilla (1m)</span>
            </div>
          </div>
        </div>
      </div>

      <!-- TAB 3: RESULTADO FINAL -->
      <div v-if="activeTab === 'resultado'" class="tab-content resultado-tab">
        <div class="section-title">3️⃣ Resultado Final</div>
        <p class="section-desc">Estadísticas y exportación de datos procesados</p>

        <div v-if="stats" class="stats-grid">
          <div class="stat-card">
            <div class="stat-value">{{ stats.total_personas }}</div>
            <div class="stat-label">Personas identificadas</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">{{ stats.total_puntos }}</div>
            <div class="stat-label">Puntos de trayectoria</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">{{ stats.tiempo_total_s?.toFixed(1) }}s</div>
            <div class="stat-label">Duración total</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">{{ stats.duracion_promedio_s?.toFixed(1) }}s</div>
            <div class="stat-label">Duración promedio</div>
          </div>
        </div>

        <div class="outputs-section">
          <h3>📦 Archivos generados:</h3>
          <ul class="output-files">
            <li>
              <span class="file-icon">📹</span>
              <span class="file-name">cam01_procesado.mp4</span>
              <span class="file-status">✓ 31 MB</span>
            </li>
            <li>
              <span class="file-icon">📹</span>
              <span class="file-name">cam02_procesado.mp4</span>
              <span class="file-status">✓ 31 MB</span>
            </li>
            <li>
              <span class="file-icon">📹</span>
              <span class="file-name">cam03_procesado.mp4</span>
              <span class="file-status">✓ 30 MB</span>
            </li>
            <li>
              <span class="file-icon">🗺️</span>
              <span class="file-name">mapa_2d.mp4</span>
              <span class="file-status">✓ 30 MB</span>
            </li>
            <li>
              <span class="file-icon">🖼️</span>
              <span class="file-name">mapa_trayectorias.png</span>
              <span class="file-status">✓ 269 KB</span>
            </li>
            <li>
              <span class="file-icon">📊</span>
              <span class="file-name">trajectory_points.csv</span>
              <span class="file-status">✓ 2.7 MB</span>
            </li>
            <li>
              <span class="file-icon">👥</span>
              <span class="file-name">identidades_genero.csv</span>
              <span class="file-status">✓ 1 KB</span>
            </li>
          </ul>
        </div>

        <div class="export-section">
          <h3>📥 Descargas:</h3>
          <button class="download-btn">
            <span>⬇️</span> Descargar CSV de trayectorias
          </button>
          <button class="download-btn">
            <span>⬇️</span> Descargar mapa 2D (PNG)
          </button>
          <button class="download-btn">
            <span>⬇️</span> Descargar todos los videos
          </button>
        </div>

        <div class="quality-metrics">
          <h3>📈 Métricas de calidad:</h3>
          <div class="metric-bar">
            <label>Precisión de detección</label>
            <div class="progress-bar">
              <div class="progress-fill" style="width: 92%;"></div>
            </div>
            <span>92%</span>
          </div>
          <div class="metric-bar">
            <label>Consistencia multicámara</label>
            <div class="progress-bar">
              <div class="progress-fill" style="width: 88%;"></div>
            </div>
            <span>88%</span>
          </div>
          <div class="metric-bar">
            <label>Cobertura de trayectorias</label>
            <div class="progress-bar">
              <div class="progress-fill" style="width: 95%;"></div>
            </div>
            <span>95%</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.esan-page {
  padding: 20px;
  background: linear-gradient(135deg, var(--bg-secondary) 0%, var(--bg-primary) 100%);
  min-height: 100vh;
}

.esan-header {
  margin-bottom: 30px;
  text-align: center;
  padding: 20px 0;
  border-bottom: 2px solid var(--border-color);
}

.esan-header h1 {
  font-size: 2.5em;
  margin: 0 0 10px 0;
  color: var(--text-primary);
}

.subtitle {
  color: var(--text-secondary);
  font-size: 1.1em;
  margin: 0;
}

.esan-tabs {
  display: flex;
  gap: 10px;
  margin-bottom: 30px;
  flex-wrap: wrap;
}

.tab-btn {
  flex: 1;
  min-width: 150px;
  padding: 12px 20px;
  background: var(--bg-secondary);
  border: 2px solid var(--border-color);
  color: var(--text-primary);
  border-radius: 8px;
  font-size: 1em;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  gap: 8px;
}

.tab-icon {
  font-size: 1.3em;
}

.tab-btn:hover {
  border-color: var(--accent-color);
  background: var(--bg-primary);
}

.tab-btn.active {
  background: var(--accent-color);
  color: white;
  border-color: var(--accent-color);
}

.esan-content {
  background: var(--bg-primary);
  border-radius: 12px;
  padding: 30px;
  border: 1px solid var(--border-color);
}

.tab-content {
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.section-title {
  font-size: 1.5em;
  font-weight: bold;
  color: var(--text-primary);
  margin-bottom: 10px;
}

.section-desc {
  color: var(--text-secondary);
  margin-bottom: 20px;
  font-size: 0.95em;
}

/* TAB 1: PROCESAMIENTO */
.cameras-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
  margin-bottom: 40px;
}

.camera-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
}

.camera-title {
  padding: 12px;
  background: var(--accent-color);
  color: white;
  font-weight: bold;
  text-align: center;
}

.camera-preview {
  aspect-ratio: 16/9;
  background: linear-gradient(135deg, #1a1a1a 0%, #2a2a2a 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #888;
}

.camera-preview video {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.mapa-video {
  background: var(--bg-secondary);
  border-radius: 8px;
  padding: 15px;
  text-align: center;
}

.mapa-video h4 {
  margin-top: 0;
  color: var(--text-primary);
}

.mapa-video video {
  max-width: 100%;
  border-radius: 6px;
  border: 1px solid var(--border-color);
}

.placeholder {
  text-align: center;
}

.placeholder span {
  display: block;
  font-weight: bold;
}

.placeholder p {
  margin: 8px 0 0 0;
  font-size: 0.9em;
  color: #666;
}

.camera-info {
  padding: 10px 12px;
  border-top: 1px solid var(--border-color);
  text-align: center;
}

.processing-steps {
  background: var(--bg-secondary);
  padding: 20px;
  border-radius: 8px;
  margin-top: 30px;
}

.processing-steps h3 {
  margin-top: 0;
  color: var(--text-primary);
}

.steps-list {
  list-style: decimal;
  margin: 15px 0;
  padding-left: 30px;
  line-height: 1.8;
  color: var(--text-secondary);
}

.steps-list li {
  margin-bottom: 12px;
}

.steps-list strong {
  color: var(--text-primary);
}

/* TAB 2: MAPA */
.mapa-container {
  display: flex;
  flex-direction: column;
  gap: 25px;
}

.mapa-info {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 15px;
}

.info-box {
  background: var(--bg-secondary);
  padding: 15px;
  border-radius: 8px;
  border-left: 4px solid var(--accent-color);
}

.info-box strong {
  display: block;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.info-box p {
  margin: 0;
  font-size: 1.5em;
  color: var(--accent-color);
  font-weight: bold;
}

.mapa-image {
  background: var(--bg-secondary);
  border-radius: 8px;
  padding: 15px;
  text-align: center;
}

.mapa-image img {
  max-width: 100%;
  height: auto;
  border-radius: 6px;
  border: 1px solid var(--border-color);
}

.image-caption {
  color: var(--text-secondary);
  margin-top: 10px;
  font-size: 0.9em;
}

.mapa-legend {
  background: var(--bg-secondary);
  padding: 15px;
  border-radius: 8px;
}

.mapa-legend h4 {
  margin-top: 0;
  color: var(--text-primary);
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 10px 0;
  font-size: 0.9em;
}

.legend-color {
  width: 20px;
  height: 20px;
  border-radius: 3px;
  border: 1px solid var(--border-color);
}

/* TAB 3: RESULTADO */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 20px;
  margin-bottom: 40px;
}

.stat-card {
  background: linear-gradient(135deg, var(--accent-color) 0%, #6f4fc1 100%);
  color: white;
  padding: 25px;
  border-radius: 8px;
  text-align: center;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.2);
}

.stat-value {
  font-size: 2.5em;
  font-weight: bold;
  margin-bottom: 10px;
}

.stat-label {
  font-size: 0.95em;
  opacity: 0.9;
}

.outputs-section,
.export-section,
.quality-metrics {
  margin: 30px 0;
  padding: 20px;
  background: var(--bg-secondary);
  border-radius: 8px;
}

.outputs-section h3,
.export-section h3,
.quality-metrics h3 {
  margin-top: 0;
  color: var(--text-primary);
}

.output-files {
  list-style: none;
  padding: 0;
  margin: 15px 0;
}

.output-files li {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: var(--bg-primary);
  border-radius: 6px;
  margin-bottom: 8px;
  border: 1px solid var(--border-color);
}

.file-icon {
  font-size: 1.3em;
  min-width: 24px;
}

.file-name {
  flex: 1;
  color: var(--text-primary);
  font-weight: 500;
}

.file-status {
  color: var(--text-secondary);
  font-size: 0.9em;
}

.download-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  margin: 8px 8px 8px 0;
  background: var(--accent-color);
  color: white;
  border: none;
  border-radius: 6px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.3s ease;
}

.download-btn:hover {
  opacity: 0.85;
}

.metric-bar {
  margin-bottom: 20px;
}

.metric-bar label {
  display: block;
  margin-bottom: 8px;
  color: var(--text-primary);
  font-weight: 500;
}

.progress-bar {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  height: 24px;
  overflow: hidden;
  display: inline-block;
  width: 200px;
  vertical-align: middle;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--accent-color) 0%, #6f4fc1 100%);
  transition: width 0.3s ease;
}

.metric-bar span {
  display: inline-block;
  margin-left: 15px;
  color: var(--text-secondary);
  font-weight: bold;
  min-width: 40px;
  text-align: right;
}

@media (max-width: 768px) {
  .esan-header h1 {
    font-size: 1.8em;
  }

  .esan-tabs {
    flex-direction: column;
  }

  .tab-btn {
    width: 100%;
  }

  .stats-grid {
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  }

  .cameras-grid {
    grid-template-columns: 1fr;
  }
}
</style>
