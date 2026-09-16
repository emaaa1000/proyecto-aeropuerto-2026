CREATE TABLE IF NOT EXISTS analysis_sessions (
    session_id UUID PRIMARY KEY,
    recording_start TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    mode VARCHAR(20) NOT NULL CHECK (mode IN ('VISUAL_TEMPORAL', 'CALIBRADO')),
    config_version VARCHAR(20),
    config_sha256 CHAR(64),
    status VARCHAR(10) NOT NULL DEFAULT 'RUNNING' CHECK (status IN ('RUNNING', 'DONE', 'FAILED')),
    CHECK (ended_at IS NULL OR ended_at >= started_at)
);
