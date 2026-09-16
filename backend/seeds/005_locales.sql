INSERT INTO locales (local_id, name, category) VALUES
    (1, 'Café Andino', 'GASTRONOMIA'),
    (2, 'Lima Duty Free', 'RETAIL'),
    (3, 'Punto de información', 'SERVICIOS')
ON CONFLICT DO NOTHING;

SELECT setval(pg_get_serial_sequence('locales', 'local_id'), (SELECT max(local_id) FROM locales));
