-- Contraseña de prueba para todos los usuarios: Demo1234!
-- Hash generado con golang.org/x/crypto/bcrypt (DefaultCost=10), la misma
-- librería que ya trae el backend como dependencia indirecta.
INSERT INTO users (user_id, name, email, password_hash) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Admin Demo', 'admin@aerovision.demo', '$2a$10$HG4.o2KfZ9NA1Pb4F/JWKOu5.kQdLTI6tdbdWCb1wy6TOUJOtIZ9u'),
    ('a0000000-0000-0000-0000-000000000002', 'Ana Torres', 'ana.torres@aerovision.demo', '$2a$10$HG4.o2KfZ9NA1Pb4F/JWKOu5.kQdLTI6tdbdWCb1wy6TOUJOtIZ9u'),
    ('a0000000-0000-0000-0000-000000000003', 'Luis Ramos', 'luis.ramos@aerovision.demo', '$2a$10$HG4.o2KfZ9NA1Pb4F/JWKOu5.kQdLTI6tdbdWCb1wy6TOUJOtIZ9u'),
    ('a0000000-0000-0000-0000-000000000004', 'Carla Vidal', 'carla.vidal@aerovision.demo', '$2a$10$HG4.o2KfZ9NA1Pb4F/JWKOu5.kQdLTI6tdbdWCb1wy6TOUJOtIZ9u')
ON CONFLICT (user_id) DO NOTHING;

-- No existe rol ANALYST/VIEWER en el esquema (solo ADMIN y COMMERCIAL_USER,
-- ver seeds/004_roles.sql); los usuarios no-admin usan COMMERCIAL_USER.
INSERT INTO user_roles (user_id, role_id)
SELECT u.user_id, r.role_id FROM users u, roles r
WHERE r.name = 'ADMIN' AND u.email = 'admin@aerovision.demo'
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT u.user_id, r.role_id FROM users u, roles r
WHERE r.name = 'COMMERCIAL_USER'
  AND u.email IN ('ana.torres@aerovision.demo', 'luis.ramos@aerovision.demo', 'carla.vidal@aerovision.demo')
ON CONFLICT DO NOTHING;
