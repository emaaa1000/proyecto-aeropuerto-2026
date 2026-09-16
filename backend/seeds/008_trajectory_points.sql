WITH people AS (
    SELECT n,
           ('00000000-0000-4000-8000-' || lpad(n::text, 12, '0'))::uuid AS global_id,
           now() - interval '52 minutes' + n * interval '2 minutes' AS started_at
    FROM generate_series(1, 14) AS n
), observations AS (
    SELECT p.global_id, p.started_at, s.step,
           CASE WHEN s.step < 4 THEN 1 WHEN s.step < 12 THEN 2 WHEN s.step < 16 THEN 3 ELSE 4 END AS camera_id,
           CASE WHEN s.step < 4 THEN 2 WHEN s.step < 12 THEN 1 WHEN s.step < 16 THEN 3 ELSE 4 END AS zone_id,
           CASE
             WHEN s.step < 4 THEN 248 + s.step * 18
             WHEN s.step < 12 THEN 265 + (s.step - 4) * 4
             WHEN s.step < 16 THEN 265 + (s.step - 12) * 18
             ELSE 312 + ((s.step - 16) % 2) * 8
           END + (p.n % 3) * 1.2 AS x,
           CASE
             WHEN s.step < 4 THEN 600
             WHEN s.step < 12 THEN 620 + (p.n % 4) * 7
             WHEN s.step < 16 THEN 680 + (p.n % 4) * 4
             ELSE 618 + ((s.step - 16) % 2) * 10
           END AS y,
           CASE WHEN s.step BETWEEN 7 AND 10 THEN 0.08 ELSE 1.15 END::real AS speed,
           CASE WHEN s.step < 12 THEN 90 WHEN s.step < 16 THEN 180 ELSE 270 END::real AS direction
    FROM people p CROSS JOIN generate_series(0, 19) AS s(step)
)
INSERT INTO trajectory_points (global_id, timestamp, camera_id, x, y, zone_id, speed, direction, confidence)
SELECT global_id, started_at + step * interval '5 seconds', camera_id, x, y, zone_id, speed, direction, 0.94
FROM observations
WHERE NOT EXISTS (SELECT 1 FROM trajectory_points);
