CREATE TABLE IF NOT EXISTS spatial_events (
    event_id BIGSERIAL PRIMARY KEY,
    global_id UUID NOT NULL REFERENCES identities(global_id) ON DELETE CASCADE,
    zone_id INTEGER NOT NULL REFERENCES aero_zones(zone_id) ON DELETE CASCADE,
    event_type VARCHAR(20) NOT NULL CHECK (event_type IN ('EXPOSURE', 'ENTER', 'DWELL', 'EXIT', 'RETURN', 'QUEUE')),
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ,
    duration_s REAL GENERATED ALWAYS AS (EXTRACT(EPOCH FROM (end_time - start_time))) STORED,
    confidence REAL CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
    CHECK (end_time IS NULL OR end_time >= start_time)
);

CREATE INDEX IF NOT EXISTS idx_events_global_time ON spatial_events (global_id, start_time);
CREATE INDEX IF NOT EXISTS idx_events_zone_time ON spatial_events (zone_id, start_time);
