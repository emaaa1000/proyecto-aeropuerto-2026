CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS zones (
    id text PRIMARY KEY,
    floor_id integer REFERENCES floors,
    name text NOT NULL,
    kind text NOT NULL,
    geom geometry(Polygon,0) NOT NULL
);

CREATE INDEX IF NOT EXISTS zones_geom ON zones USING gist(geom);
