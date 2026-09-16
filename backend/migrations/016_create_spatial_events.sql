CREATE TABLE IF NOT EXISTS spatial_events (
    event_id BIGSERIAL PRIMARY KEY,
    global_id UUID NOT NULL,
    zone_id INTEGER NOT NULL,
    event_type VARCHAR(20) NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ,
    duration_s REAL,
    confidence REAL,
    CONSTRAINT fk_event_zone FOREIGN KEY (zone_id) REFERENCES aero_zones(zone_id) ON DELETE CASCADE,
    CONSTRAINT chk_event_type CHECK (event_type IN ('EXPOSURE', 'ENTER', 'DWELL', 'EXIT', 'RETURN', 'QUEUE')),
    CONSTRAINT chk_event_time CHECK (end_time IS NULL OR end_time >= start_time),
    CONSTRAINT chk_event_duration CHECK (duration_s IS NULL OR duration_s >= 0),
    CONSTRAINT chk_event_confidence CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1))
);

CREATE INDEX IF NOT EXISTS idx_events_global_time ON spatial_events (global_id, start_time);
CREATE INDEX IF NOT EXISTS idx_events_zone_time ON spatial_events (zone_id, start_time);
CREATE INDEX IF NOT EXISTS idx_events_type ON spatial_events (event_type);
