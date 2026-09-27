-- Sitio ESAN: su plano (mapa_piso.json + camaras.json del modelo LAP01), sus
-- cámaras y las sesiones del modelo (build del dataset o cámara en vivo).
INSERT INTO floors (id, name) VALUES (100, 'ESAN') ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS esan_maps (
    floor_id INTEGER PRIMARY KEY REFERENCES floors(id),
    config JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE analysis_sessions ADD COLUMN IF NOT EXISTS floor_id INTEGER REFERENCES floors(id);
ALTER TABLE analysis_sessions ADD COLUMN IF NOT EXISTS kind VARCHAR(10) NOT NULL DEFAULT 'BUILD';
ALTER TABLE analysis_sessions ADD COLUMN IF NOT EXISTS name VARCHAR(120);
ALTER TABLE analysis_sessions ADD COLUMN IF NOT EXISTS summary JSONB;
ALTER TABLE analysis_sessions DROP CONSTRAINT IF EXISTS analysis_sessions_kind_check;
ALTER TABLE analysis_sessions ADD CONSTRAINT analysis_sessions_kind_check CHECK (kind IN ('BUILD', 'LIVE'));
CREATE INDEX IF NOT EXISTS idx_sessions_floor_time ON analysis_sessions (floor_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_identities_session ON identities (session_id);

INSERT INTO aero_cameras (camera_id, name, floor_id, mode, active)
VALUES ('esan-movil', 'Cámara del teléfono', 100, 'VISUAL_TEMPORAL', TRUE)
ON CONFLICT (camera_id) DO NOTHING;
