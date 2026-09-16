CREATE TABLE IF NOT EXISTS tracklets (
    tracklet_id UUID PRIMARY KEY,
    global_id UUID NOT NULL REFERENCES identities(global_id) ON DELETE CASCADE,
    camera_id VARCHAR(30) NOT NULL REFERENCES aero_cameras(camera_id),
    local_id INTEGER NOT NULL,
    t_start TIMESTAMPTZ NOT NULL,
    t_end TIMESTAMPTZ NOT NULL,
    n_reid_views INTEGER NOT NULL DEFAULT 0 CHECK (n_reid_views >= 0),
    CHECK (t_end >= t_start)
);

CREATE INDEX IF NOT EXISTS idx_tracklets_global ON tracklets (global_id);
CREATE INDEX IF NOT EXISTS idx_tracklets_camera_time ON tracklets (camera_id, t_start);
