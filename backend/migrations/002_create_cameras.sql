CREATE TABLE IF NOT EXISTS cameras (
    id text PRIMARY KEY,
    floor_id integer REFERENCES floors,
    name text NOT NULL
);
