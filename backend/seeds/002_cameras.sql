INSERT INTO cameras (id, floor_id, name) VALUES
    ('SIM-01', 3, 'Fuente simulada')
ON CONFLICT DO NOTHING;
