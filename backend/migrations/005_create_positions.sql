CREATE TABLE IF NOT EXISTS positions (
    session_id text REFERENCES sessions,
    observed_at timestamptz NOT NULL,
    camera_id text REFERENCES cameras,
    geom geometry(Point,0) NOT NULL,
    PRIMARY KEY (session_id, observed_at)
);

CREATE INDEX IF NOT EXISTS positions_time ON positions(observed_at);
CREATE INDEX IF NOT EXISTS positions_session_time ON positions(session_id, observed_at);
