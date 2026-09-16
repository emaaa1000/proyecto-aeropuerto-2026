CREATE TABLE IF NOT EXISTS visits (
    id bigserial PRIMARY KEY,
    session_id text REFERENCES sessions,
    zone_id text REFERENCES zones,
    entered_at timestamptz NOT NULL,
    exited_at timestamptz,
    status text NOT NULL DEFAULT 'open'
);

CREATE UNIQUE INDEX IF NOT EXISTS visits_open ON visits(session_id, zone_id) WHERE status='open';
CREATE INDEX IF NOT EXISTS visits_time ON visits(entered_at);
CREATE INDEX IF NOT EXISTS visits_session_time ON visits(session_id, entered_at);
