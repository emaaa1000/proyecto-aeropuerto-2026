WITH people AS (
    SELECT ('00000000-0000-4000-8000-' || lpad(n::text, 12, '0'))::uuid AS global_id,
           now() - interval '52 minutes' + n * interval '2 minutes' AS started_at
    FROM generate_series(1, 14) AS n
    WHERE NOT EXISTS (SELECT 1 FROM spatial_events)
)
INSERT INTO spatial_events (global_id, zone_id, event_type, start_time, end_time, duration_s, confidence)
SELECT global_id, 2, 'EXPOSURE', started_at + interval '15 seconds', NULL::timestamptz, NULL::real, 0.94 FROM people
UNION ALL
SELECT global_id, 1, 'ENTER', started_at + interval '20 seconds', NULL::timestamptz, NULL::real, 0.94 FROM people
UNION ALL
SELECT global_id, 1, 'DWELL', started_at + interval '25 seconds', started_at + interval '55 seconds', 30, 0.92 FROM people
UNION ALL
SELECT global_id, 1, 'EXIT', started_at + interval '60 seconds', NULL::timestamptz, NULL::real, 0.94 FROM people
UNION ALL
SELECT global_id, 4, 'QUEUE', started_at + interval '80 seconds', started_at + interval '95 seconds', 15, 0.90 FROM people;
