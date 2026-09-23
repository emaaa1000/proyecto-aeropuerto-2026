INSERT INTO locales (local_id, name, category) VALUES
    (1, 'Duty Free Lima', 'Duty Free'),
    (2, 'Farmacia InkaFarma', 'Farmacia'),
    (3, 'Renzo Costa (Moda)', 'Retail Moda'),
    (4, 'Librería Crisol', 'Librería'),
    (5, 'Joyería Perú Gold', 'Joyería'),
    (6, 'TecnoPerú Electrónica', 'Electrónica'),
    (7, 'Starbucks', 'Café'),
    (8, 'KFC', 'Comida Rápida'),
    (9, 'La Mar Cebichería', 'Restaurante'),
    (10, 'BCP - Banco de Crédito', 'Servicios Financieros'),
    (11, 'Avis Rent a Car', 'Alquiler de Autos'),
    (12, 'Wayra Souvenirs', 'Souvenirs')
ON CONFLICT (local_id) DO NOTHING;
