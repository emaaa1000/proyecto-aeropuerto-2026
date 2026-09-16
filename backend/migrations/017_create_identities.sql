CREATE TABLE IF NOT EXISTS identities (
    global_id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES analysis_sessions(session_id) ON DELETE CASCADE,
    public_number INTEGER NOT NULL CHECK (public_number > 0),
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    n_cameras SMALLINT NOT NULL DEFAULT 1 CHECK (n_cameras > 0),
    gender_estimate VARCHAR(15) NOT NULL DEFAULT 'SIN_DETERMINAR' CHECK (gender_estimate IN ('HOMBRE', 'MUJER', 'SIN_DETERMINAR')),
    gender_confidence REAL CHECK (gender_confidence IS NULL OR gender_confidence BETWEEN 0 AND 1),
    gender_votes INTEGER NOT NULL DEFAULT 0 CHECK (gender_votes >= 0),
    UNIQUE (session_id, public_number),
    CHECK (last_seen >= first_seen),
    CHECK (gender_estimate = 'SIN_DETERMINAR' OR gender_confidence IS NOT NULL)
);
