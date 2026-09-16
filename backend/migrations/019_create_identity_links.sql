CREATE TABLE IF NOT EXISTS identity_links (
    link_id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES analysis_sessions(session_id) ON DELETE CASCADE,
    tracklet_id UUID REFERENCES tracklets(tracklet_id) ON DELETE CASCADE,
    from_global UUID,
    to_global UUID,
    action VARCHAR(15) NOT NULL CHECK (action IN ('ASSOCIATE', 'SEPARATE', 'SPLIT', 'MAP_MERGE', 'MAP_SEPARATE')),
    score REAL,
    decided_at TIMESTAMPTZ
);
