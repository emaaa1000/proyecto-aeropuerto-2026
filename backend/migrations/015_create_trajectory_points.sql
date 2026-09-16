CREATE TABLE IF NOT EXISTS trajectory_points (
    point_id BIGSERIAL PRIMARY KEY,
    global_id UUID NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    camera_id INTEGER,
    x DOUBLE PRECISION NOT NULL,
    y DOUBLE PRECISION NOT NULL,
    zone_id INTEGER,
    speed REAL,
    direction REAL,
    confidence REAL,
    CONSTRAINT fk_trajectory_camera FOREIGN KEY (camera_id) REFERENCES aero_cameras(camera_id) ON DELETE SET NULL,
    CONSTRAINT fk_trajectory_zone FOREIGN KEY (zone_id) REFERENCES aero_zones(zone_id) ON DELETE SET NULL,
    CONSTRAINT chk_trajectory_speed CHECK (speed IS NULL OR speed >= 0),
    CONSTRAINT chk_trajectory_direction CHECK (direction IS NULL OR (direction >= 0 AND direction < 360)),
    CONSTRAINT chk_trajectory_confidence CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1))
);

CREATE INDEX IF NOT EXISTS idx_trajectory_global_time ON trajectory_points (global_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_trajectory_zone_time ON trajectory_points (zone_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_trajectory_timestamp ON trajectory_points (timestamp);
