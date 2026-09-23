-- Mismos valores que identities.gender_estimate (esquema AeroVision,
-- migrations/017_create_identities.sql) por consistencia, aunque esta
-- columna vive en el esquema legado que sí lee /insights.
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS gender VARCHAR(15) NOT NULL DEFAULT 'SIN_DETERMINAR'
    CHECK (gender IN ('HOMBRE', 'MUJER', 'SIN_DETERMINAR'));
