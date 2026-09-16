CREATE TABLE IF NOT EXISTS trajectory_points (
    point_id BIGSERIAL PRIMARY KEY,
    global_id UUID NOT NULL REFERENCES identities(global_id) ON DELETE CASCADE,
    tracklet_id UUID NOT NULL REFERENCES tracklets(tracklet_id) ON DELETE CASCADE,
    camera_id VARCHAR(30) NOT NULL REFERENCES aero_cameras(camera_id),
    local_id INTEGER NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL,
    x DOUBLE PRECISION,
    y DOUBLE PRECISION,
    geom geometry(Point, 0) GENERATED ALWAYS AS (ST_SetSRID(ST_MakePoint(x, y), 0)) STORED,
    zone_id INTEGER REFERENCES aero_zones(zone_id) ON DELETE SET NULL,
    speed_mps REAL CHECK (speed_mps IS NULL OR speed_mps >= 0),
    direction_deg REAL CHECK (direction_deg IS NULL OR (direction_deg >= 0 AND direction_deg < 360)),
    confidence REAL NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    CHECK ((x IS NULL) = (y IS NULL))
);

CREATE INDEX IF NOT EXISTS idx_tp_global_time ON trajectory_points (global_id, "timestamp");
CREATE INDEX IF NOT EXISTS idx_tp_zone_time ON trajectory_points (zone_id, "timestamp");
CREATE INDEX IF NOT EXISTS idx_tp_time_brin ON trajectory_points USING BRIN ("timestamp");
CREATE INDEX IF NOT EXISTS idx_tp_geom ON trajectory_points USING GIST (geom);

CREATE OR REPLACE FUNCTION assign_zones(p_session UUID) RETURNS BIGINT
LANGUAGE sql AS $$
    WITH upd AS (
        UPDATE trajectory_points tp
           SET zone_id = (SELECT z.zone_id FROM aero_zones z
                          WHERE z.active AND ST_Covers(z.geom, tp.geom)
                          ORDER BY (z.zone_type = 'INTERIOR') DESC, z.area_m2
                          LIMIT 1)
          FROM identities i
         WHERE i.global_id = tp.global_id AND i.session_id = p_session AND tp.geom IS NOT NULL
        RETURNING 1)
    SELECT count(*) FROM upd;
$$;
