INSERT INTO aero_cameras (camera_id, name, x, y, angle_deg) VALUES
    (1, 'CAM-N3-ENTRADA-01', 245, 590, 180),
    (2, 'CAM-N3-LOCAL-01', 280, 680, 0),
    (3, 'CAM-N3-COLA-01', 340, 625, 270),
    (4, 'CAM-N3-SALIDA-01', 320, 620, 90)
ON CONFLICT DO NOTHING;

SELECT setval(pg_get_serial_sequence('aero_cameras', 'camera_id'), (SELECT max(camera_id) FROM aero_cameras));
