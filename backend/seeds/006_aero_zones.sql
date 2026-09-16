INSERT INTO aero_zones (zone_id, local_id, name, zone_type, area_m2, geom) VALUES
    (1, 1, 'Café Andino · interior', 'INTERIOR', 2250, ST_GeomFromText('POLYGON((260 610,305 610,305 660,260 660,260 610))', 0)),
    (2, 1, 'Café Andino · frente comercial', 'FRONTAGE', 720, ST_GeomFromText('POLYGON((245 596,325 596,325 605,245 605,245 596))', 0)),
    (3, NULL, 'Pasillo central', 'PASILLO', 7200, ST_GeomFromText('POLYGON((225 665,345 665,345 725,225 725,225 665))', 0)),
    (4, NULL, 'Zona de cola', 'COLA', 900, ST_GeomFromText('POLYGON((306 610,336 610,336 640,306 640,306 610))', 0)),
    (5, 3, 'Punto de información', 'OTRO', 900, ST_GeomFromText('POLYGON((220 610,250 610,250 640,220 640,220 610))', 0))
ON CONFLICT DO NOTHING;

SELECT setval(pg_get_serial_sequence('aero_zones', 'zone_id'), (SELECT max(zone_id) FROM aero_zones));
