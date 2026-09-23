-- Zona adicional para el demo de "En vivo": un patio de comidas cerca del
-- local demo ('shop'/'front' en 003_zones.sql). Forma tomada del poligono
-- que el usuario ya dibujo en map_objects ("Patio de Comidas"), convertida
-- a metros del plano con la misma formula de web/src/plan.ts (toPlan) y
-- desplazada (~70m oeste, ~40m norte) para no superponerse con 'shop' ni
-- cruzar el pasillo por el que ya caminan las sesiones sinteticas de
-- 009_positions.sql (verificado con ST_Intersects).
INSERT INTO zones (id, floor_id, name, kind, geom) VALUES
    ('food_court', 3, 'Patio de Comidas', 'food_court',
     ST_GeomFromText('POLYGON((198.08 689.33,221.46 685.04,234.34 669.77,227.18 640.67,205.24 635.90,184.72 645.44,174.70 665.00,176.13 674.54,198.08 689.33))', 0))
ON CONFLICT DO NOTHING;
