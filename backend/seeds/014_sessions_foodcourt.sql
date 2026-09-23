-- Sesiones sinteticas adicionales: comensales del patio de comidas
-- (backend/seeds/013_zones_foodcourt.sql).
INSERT INTO sessions (id, started_at, ended_at, status) VALUES
    ('trip-031', '2026-09-06T19:41:28.855Z', '2026-09-06T19:46:33.458Z', 'complete'),
    ('trip-032', '2026-09-06T19:42:34.744Z', '2026-09-06T19:46:34.437Z', 'complete'),
    ('trip-033', '2026-09-06T19:43:50.452Z', '2026-09-06T19:49:17.496Z', 'complete'),
    ('trip-034', '2026-09-06T19:45:34.208Z', '2026-09-06T19:51:03.545Z', 'complete'),
    ('trip-035', '2026-09-06T19:47:16.207Z', '2026-09-06T19:53:48.668Z', 'complete'),
    ('trip-036', '2026-09-06T19:48:22.062Z', '2026-09-06T19:53:23.023Z', 'complete'),
    ('trip-037', '2026-09-06T19:49:40.000Z', '2026-09-06T19:57:18.604Z', 'complete'),
    ('trip-038', '2026-09-06T19:51:18.999Z', '2026-09-06T19:54:34.691Z', 'complete'),
    ('trip-039', '2026-09-06T19:52:29.437Z', '2026-09-06T19:58:40.481Z', 'complete'),
    ('trip-040', '2026-09-06T19:53:46.806Z', '2026-09-06T19:59:16.143Z', 'complete'),
    ('trip-041', '2026-09-06T19:55:13.212Z', '2026-09-06T20:00:39.673Z', 'complete')
ON CONFLICT (id) DO NOTHING;
