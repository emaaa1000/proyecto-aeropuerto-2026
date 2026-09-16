CREATE TABLE IF NOT EXISTS camera_links (
    from_camera VARCHAR(30) NOT NULL REFERENCES aero_cameras(camera_id) ON DELETE CASCADE,
    to_camera VARCHAR(30) NOT NULL REFERENCES aero_cameras(camera_id) ON DELETE CASCADE,
    overlap BOOLEAN NOT NULL DEFAULT FALSE,
    t_min_s REAL NOT NULL DEFAULT 0 CHECK (t_min_s >= 0),
    t_max_s REAL NOT NULL CHECK (t_max_s > 0),
    max_distance_m REAL CHECK (max_distance_m IS NULL OR max_distance_m > 0),
    min_direction_cos REAL CHECK (min_direction_cos IS NULL OR min_direction_cos BETWEEN -1 AND 1),
    PRIMARY KEY (from_camera, to_camera),
    CHECK (from_camera <> to_camera),
    CHECK (t_max_s >= t_min_s)
);
