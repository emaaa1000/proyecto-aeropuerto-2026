-- 005_locales.sql y 006_aero_zones.sql insertan local_id/zone_id explicitos
-- (para que el seed sea legible y reproducible), lo que deja las secuencias
-- SERIAL de ambas tablas ancladas en 1. Cualquier INSERT posterior hecho por
-- la aplicacion (sin id explicito) choca entonces con una fila ya sembrada.
-- Resincroniza ambas secuencias al maximo id ya usado.
SELECT setval(pg_get_serial_sequence('locales', 'local_id'), COALESCE((SELECT MAX(local_id) FROM locales), 1));
SELECT setval(pg_get_serial_sequence('aero_zones', 'zone_id'), COALESCE((SELECT MAX(zone_id) FROM aero_zones), 1));
