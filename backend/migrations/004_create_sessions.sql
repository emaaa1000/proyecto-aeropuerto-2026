CREATE TABLE IF NOT EXISTS sessions (
    id text PRIMARY KEY,
    started_at timestamptz NOT NULL,
    ended_at timestamptz,
    status text NOT NULL DEFAULT 'active'
);
