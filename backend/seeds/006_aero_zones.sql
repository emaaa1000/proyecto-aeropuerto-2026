-- zone_type = 'INTERIOR': footprint del local (la categoría de negocio ya vive en locales.category).
-- area_m2 es columna generada (ST_Area(geom)) y no se inserta explícitamente.
INSERT INTO aero_zones (zone_id, local_id, floor_id, name, zone_type, geom) VALUES
    (1, 1, 3, 'Zona - Duty Free Lima', 'INTERIOR', ST_GeomFromText('POLYGON((3 14, 14 14, 14 18, 3 18, 3 14))', 0)),
    (2, 2, 3, 'Zona - Farmacia InkaFarma', 'INTERIOR', ST_GeomFromText('POLYGON((16 14, 25 14, 25 18, 16 18, 16 14))', 0)),
    (3, 3, 3, 'Zona - Renzo Costa (Moda)', 'INTERIOR', ST_GeomFromText('POLYGON((27 14, 34 14, 34 18, 27 18, 27 14))', 0)),
    (4, 4, 3, 'Zona - Librería Crisol', 'INTERIOR', ST_GeomFromText('POLYGON((36 14, 45 14, 45 18, 36 18, 36 14))', 0)),
    (5, 5, 3, 'Zona - Joyería Perú Gold', 'INTERIOR', ST_GeomFromText('POLYGON((47 14, 55 14, 55 18, 47 18, 47 14))', 0)),
    (6, 6, 3, 'Zona - TecnoPerú Electrónica', 'INTERIOR', ST_GeomFromText('POLYGON((57 14, 64 14, 64 18, 57 18, 57 14))', 0)),
    (7, 7, 3, 'Zona - Starbucks', 'INTERIOR', ST_GeomFromText('POLYGON((3 2, 10 2, 10 6, 3 6, 3 2))', 0)),
    (8, 8, 3, 'Zona - KFC', 'INTERIOR', ST_GeomFromText('POLYGON((12 2, 19 2, 19 6, 12 6, 12 2))', 0)),
    (9, 9, 3, 'Zona - La Mar Cebichería', 'INTERIOR', ST_GeomFromText('POLYGON((21 2, 31 2, 31 6, 21 6, 21 2))', 0)),
    (10, 10, 3, 'Zona - BCP - Banco de Crédito', 'INTERIOR', ST_GeomFromText('POLYGON((33 2, 40 2, 40 6, 33 6, 33 2))', 0)),
    (11, 11, 3, 'Zona - Avis Rent a Car', 'INTERIOR', ST_GeomFromText('POLYGON((42 2, 49 2, 49 6, 42 6, 42 2))', 0)),
    (12, 12, 3, 'Zona - Wayra Souvenirs', 'INTERIOR', ST_GeomFromText('POLYGON((51 2, 60 2, 60 6, 51 6, 51 2))', 0))
ON CONFLICT (zone_id) DO NOTHING;
