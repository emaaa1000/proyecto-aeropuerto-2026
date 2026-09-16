CREATE TABLE IF NOT EXISTS aero_zones (
    zone_id SERIAL PRIMARY KEY,
    local_id INTEGER,
    name VARCHAR(100) NOT NULL,
    zone_type VARCHAR(30) NOT NULL,
    area_m2 DOUBLE PRECISION,
    geom geometry(Polygon) NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    CONSTRAINT fk_aero_zone_local FOREIGN KEY (local_id) REFERENCES locales(local_id) ON DELETE SET NULL,
    CONSTRAINT chk_zone_type CHECK (zone_type IN ('INTERIOR', 'FRONTAGE', 'PASILLO', 'ENTRADA', 'CHECKIN', 'SEGURIDAD', 'PUERTA', 'COLA', 'OTRO')),
    CONSTRAINT chk_zone_area CHECK (area_m2 IS NULL OR area_m2 > 0)
);

CREATE INDEX IF NOT EXISTS idx_aero_zones_geom ON aero_zones USING GIST (geom);
