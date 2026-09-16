CREATE TABLE IF NOT EXISTS aero_cameras (
    camera_id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    x DOUBLE PRECISION NOT NULL,
    y DOUBLE PRECISION NOT NULL,
    angle_deg REAL NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    CONSTRAINT chk_camera_angle CHECK (angle_deg >= 0 AND angle_deg < 360)
);
