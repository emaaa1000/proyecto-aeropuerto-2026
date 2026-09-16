CREATE TABLE IF NOT EXISTS aero_zones (
    zone_id SERIAL PRIMARY KEY,
    local_id INTEGER REFERENCES locales(local_id) ON DELETE RESTRICT,
    floor_id INTEGER NOT NULL DEFAULT 3 REFERENCES floors(id),
    name VARCHAR(100) NOT NULL,
    zone_type VARCHAR(30) NOT NULL CHECK (zone_type IN ('INTERIOR', 'FRONTAGE', 'PASILLO', 'ENTRADA', 'CHECKIN', 'SEGURIDAD', 'PUERTA', 'COLA', 'OTRO')),
    color VARCHAR(9),
    geom geometry(Polygon, 0) NOT NULL,
    area_m2 DOUBLE PRECISION GENERATED ALWAYS AS (ST_Area(geom)) STORED,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    CHECK (ST_IsValid(geom) AND NOT ST_IsEmpty(geom)),
    CHECK (zone_type NOT IN ('INTERIOR', 'FRONTAGE') OR local_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_aero_zones_geom ON aero_zones USING GIST (geom);
