CREATE TABLE IF NOT EXISTS events (
    id bigserial PRIMARY KEY,
    session_id text REFERENCES sessions,
    zone_id text REFERENCES zones,
    kind text NOT NULL,
    observed_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS events_session_time ON events(session_id, observed_at);
CREATE INDEX IF NOT EXISTS events_time ON events(observed_at);
