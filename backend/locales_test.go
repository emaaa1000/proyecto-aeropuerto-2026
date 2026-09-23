package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestValidPlanPolygon(t *testing.T) {
	inside := json.RawMessage(`{"type":"Polygon","coordinates":[[[10,10],[20,10],[20,20],[10,20],[10,10]]]}`)
	if !validPlanPolygon(inside) {
		t.Fatal("valid in-bounds polygon rejected")
	}
	outside := json.RawMessage(`{"type":"Polygon","coordinates":[[[10,10],[600,10],[600,20],[10,20],[10,10]]]}`)
	if validPlanPolygon(outside) {
		t.Fatal("out-of-bounds polygon accepted")
	}
	negative := json.RawMessage(`{"type":"Polygon","coordinates":[[[-5,10],[20,10],[20,20],[-5,20],[-5,10]]]}`)
	if validPlanPolygon(negative) {
		t.Fatal("negative coordinate accepted")
	}
	notClosed := json.RawMessage(`{"type":"Polygon","coordinates":[[[10,10],[20,10],[20,20],[10,20]]]}`)
	if validPlanPolygon(notClosed) {
		t.Fatal("unclosed ring accepted")
	}
	point := json.RawMessage(`{"type":"Point","coordinates":[10,10]}`)
	if validPlanPolygon(point) {
		t.Fatal("point accepted as polygon")
	}
}

func TestLocalesAndAeroZonesPersistence(t *testing.T) {
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("TEST_DATABASE_URL not configured")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, u)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var name string
	db.QueryRow(ctx, "SELECT current_database()").Scan(&name)
	if name != "aeropuerto_test" {
		t.Fatal("requires test database")
	}
	a := &App{db: db}
	if e = a.migrate(ctx); e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	a.localesRoutes(mux)
	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var r *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			r = httptest.NewRequest(method, path, bytes.NewReader(b))
			r.Header.Set("Content-Type", "application/json")
		} else {
			r = httptest.NewRequest(method, path, nil)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}

	var locale Locale
	if _, err := db.Exec(ctx, "INSERT INTO locales(local_id,name,category) VALUES(9001,'Local de prueba','Otros') ON CONFLICT (local_id) DO NOTHING"); err != nil {
		t.Fatal(err)
	}
	w := do("PATCH", "/api/v1/locales/9001", map[string]string{"name": "Nuevo nombre"})
	if w.Code != 200 {
		t.Fatalf("patch locale: %d %s", w.Code, w.Body)
	}
	json.Unmarshal(w.Body.Bytes(), &locale)
	if locale.Name != "Nuevo nombre" || locale.Category != "Otros" {
		t.Fatalf("partial update changed untouched field: %+v", locale)
	}

	w = do("PATCH", "/api/v1/locales/9001", map[string]string{})
	if w.Code != 400 {
		t.Fatal("empty patch accepted")
	}

	polygon := json.RawMessage(`{"type":"Polygon","coordinates":[[[1,1],[5,1],[5,5],[1,5],[1,1]]]}`)
	w = do("POST", "/api/v1/aero-zones", map[string]any{"local_id": 9001, "name": "Zona prueba", "zone_type": "INTERIOR", "geometry": polygon})
	if w.Code != 201 {
		t.Fatalf("create zone: %d %s", w.Code, w.Body)
	}
	var zone AeroZone
	json.Unmarshal(w.Body.Bytes(), &zone)
	if zone.ZoneID == 0 || zone.LocalID != 9001 {
		t.Fatalf("missing persisted zone: %+v", zone)
	}

	zoneID := strconv.Itoa(zone.ZoneID)
	newPolygon := json.RawMessage(`{"type":"Polygon","coordinates":[[[2,2],[8,2],[8,8],[2,8],[2,2]]]}`)
	w = do("PATCH", "/api/v1/aero-zones/"+zoneID, map[string]any{"geometry": newPolygon})
	if w.Code != 200 {
		t.Fatalf("patch zone geometry: %d %s", w.Code, w.Body)
	}
	var moved AeroZone
	json.Unmarshal(w.Body.Bytes(), &moved)
	if moved.LocalID != 9001 || moved.ZoneID != zone.ZoneID {
		t.Fatalf("local_id/zone_id changed via patch: %+v", moved)
	}
	if moved.AreaM2 == zone.AreaM2 {
		t.Fatal("area_m2 did not change after geometry update")
	}

	w = do("PATCH", "/api/v1/aero-zones/"+zoneID, map[string]any{"local_id": 1})
	if w.Code != 400 {
		t.Fatal("local_id field accepted in zone patch body")
	}

	w = do("POST", "/api/v1/aero-zones", map[string]any{"local_id": 999999, "name": "Huerfana", "zone_type": "INTERIOR", "geometry": polygon})
	if w.Code != 400 {
		t.Fatal("nonexistent local_id accepted")
	}

	outOfBounds := json.RawMessage(`{"type":"Polygon","coordinates":[[[10,10],[900,10],[900,20],[10,20],[10,10]]]}`)
	w = do("POST", "/api/v1/aero-zones", map[string]any{"local_id": 9001, "name": "Fuera", "zone_type": "INTERIOR", "geometry": outOfBounds})
	if w.Code != 400 {
		t.Fatal("out-of-plan polygon accepted")
	}
}
