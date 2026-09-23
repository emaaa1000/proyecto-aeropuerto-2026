package main

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Locale struct {
	LocalID  int    `json:"local_id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Active   bool   `json:"active"`
}

type AeroZone struct {
	ZoneID   int             `json:"zone_id"`
	LocalID  int             `json:"local_id"`
	FloorID  int             `json:"floor_id"`
	Name     string          `json:"name"`
	ZoneType string          `json:"zone_type"`
	Color    string          `json:"color"`
	Geometry json.RawMessage `json:"geometry"`
	AreaM2   float64         `json:"area_m2"`
	Active   bool            `json:"active"`
}

// The real plan spans [0,planWidth] x [0,planHeight] plan-meters (SRID 0),
// matching web/src/plan.json's width/height — the same coarse bounding-box
// approach map_objects.go uses for its lon/lat bbox, not exact terminal-shape
// containment (no outline polygon is stored in PostGIS to check ST_Within).
const (
	planWidth  = 546.415393798229
	planHeight = 978.4476874122217
)

var aeroZoneTypes = map[string]bool{
	"INTERIOR": true, "FRONTAGE": true, "PASILLO": true, "ENTRADA": true,
	"CHECKIN": true, "SEGURIDAD": true, "PUERTA": true, "COLA": true, "OTRO": true,
}

const aeroZoneColumns = "zone_id,local_id,floor_id,name,zone_type,COALESCE(color,''),ST_AsGeoJSON(geom)::json,area_m2,active"

func scanAeroZone(row pgx.Row, z *AeroZone) error {
	return row.Scan(&z.ZoneID, &z.LocalID, &z.FloorID, &z.Name, &z.ZoneType, &z.Color, &z.Geometry, &z.AreaM2, &z.Active)
}

// validPlanPolygon mirrors map_objects.go's validGeometry, but for aero_zones'
// SRID-0 plan-meters coordinates instead of WGS84 lon/lat.
func validPlanPolygon(raw json.RawMessage) bool {
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if json.Unmarshal(raw, &g) != nil || g.Type != "Polygon" {
		return false
	}
	var rings [][][]float64
	if json.Unmarshal(g.Coordinates, &rings) != nil || len(rings) != 1 || len(rings[0]) < 4 || len(rings[0]) > 101 {
		return false
	}
	r := rings[0]
	for _, p := range r {
		if len(p) != 2 || math.IsNaN(p[0]) || math.IsNaN(p[1]) || p[0] < 0 || p[0] > planWidth || p[1] < 0 || p[1] > planHeight {
			return false
		}
	}
	return r[0][0] == r[len(r)-1][0] && r[0][1] == r[len(r)-1][1]
}

func (a *App) localesRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/locales", a.listLocales)
	m.HandleFunc("PATCH /api/v1/locales/{id}", a.patchLocale)
	m.HandleFunc("GET /api/v1/aero-zones", a.listAeroZones)
	m.HandleFunc("POST /api/v1/aero-zones", a.createAeroZone)
	m.HandleFunc("PATCH /api/v1/aero-zones/{id}", a.patchAeroZone)
}

func (a *App) listLocales(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := a.db.Query(ctx, "SELECT local_id,name,category,active FROM locales ORDER BY name,local_id")
	if err != nil {
		fail(w, 503, "No se pudieron cargar los locales")
		return
	}
	defer rows.Close()
	result := []Locale{}
	for rows.Next() {
		var l Locale
		if rows.Scan(&l.LocalID, &l.Name, &l.Category, &l.Active) != nil {
			fail(w, 503, "No se pudieron leer los locales")
			return
		}
		result = append(result, l)
	}
	if rows.Err() != nil {
		fail(w, 503, "Lectura interrumpida")
		return
	}
	jsonResponse(w, result)
}

func (a *App) patchLocale(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, 415, "Se requiere application/json")
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		fail(w, 400, "Id inválido")
		return
	}
	var patch struct {
		Name     *string `json:"name"`
		Category *string `json:"category"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(&patch) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400, "Datos inválidos")
		return
	}
	if patch.Name == nil && patch.Category == nil {
		fail(w, 400, "No hay cambios que aplicar")
		return
	}
	if patch.Name != nil {
		n := strings.TrimSpace(*patch.Name)
		if n == "" || len(n) > 100 {
			fail(w, 400, "El nombre debe tener entre 1 y 100 caracteres")
			return
		}
		patch.Name = &n
	}
	if patch.Category != nil {
		c := strings.TrimSpace(*patch.Category)
		if len(c) > 50 {
			fail(w, 400, "La categoría no puede superar 50 caracteres")
			return
		}
		patch.Category = &c
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var saved Locale
	err = a.db.QueryRow(ctx, `UPDATE locales SET name=COALESCE($2,name), category=COALESCE($3,category)
		WHERE local_id=$1 RETURNING local_id,name,category,active`, id, patch.Name, patch.Category,
	).Scan(&saved.LocalID, &saved.Name, &saved.Category, &saved.Active)
	if err == pgx.ErrNoRows {
		fail(w, 404, "Local no encontrado")
		return
	}
	if err != nil {
		fail(w, 503, "No se pudo guardar el local")
		return
	}
	jsonResponse(w, saved)
}

func (a *App) listAeroZones(w http.ResponseWriter, r *http.Request) {
	if f := r.URL.Query().Get("floor_id"); f != "" && f != "3" {
		fail(w, 400, "Solo el nivel 3 está habilitado")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := a.db.Query(ctx, "SELECT "+aeroZoneColumns+" FROM aero_zones WHERE floor_id=3 ORDER BY name,zone_id")
	if err != nil {
		fail(w, 503, "No se pudieron cargar las zonas comerciales")
		return
	}
	defer rows.Close()
	result := []AeroZone{}
	for rows.Next() {
		var z AeroZone
		if scanAeroZone(rows, &z) != nil {
			fail(w, 503, "No se pudieron leer las zonas comerciales")
			return
		}
		result = append(result, z)
	}
	if rows.Err() != nil {
		fail(w, 503, "Lectura interrumpida")
		return
	}
	jsonResponse(w, result)
}

func (a *App) createAeroZone(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, 415, "Se requiere application/json")
		return
	}
	var in struct {
		LocalID  int             `json:"local_id"`
		Name     string          `json:"name"`
		ZoneType string          `json:"zone_type"`
		Geometry json.RawMessage `json:"geometry"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400, "Datos inválidos")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.LocalID <= 0 || in.Name == "" || len(in.Name) > 100 || !aeroZoneTypes[in.ZoneType] || !validPlanPolygon(in.Geometry) {
		fail(w, 400, "Datos inválidos: revisa local_id, nombre, tipo de zona y el polígono (dentro del plano, 3 a 100 vértices)")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var exists bool
	if err := a.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM locales WHERE local_id=$1)", in.LocalID).Scan(&exists); err != nil {
		fail(w, 503, "No se pudo validar el local")
		return
	}
	if !exists {
		fail(w, 400, "El local_id indicado no existe")
		return
	}
	// Reject crossing/invalid polygons in PostGIS before attempting to write.
	var valid bool
	if err := a.db.QueryRow(ctx, "SELECT ST_IsValid(ST_GeomFromGeoJSON($1))", string(in.Geometry)).Scan(&valid); err != nil || !valid {
		fail(w, 400, "El polígono se cruza o tiene geometría inválida")
		return
	}
	var saved AeroZone
	err := scanAeroZone(a.db.QueryRow(ctx, `INSERT INTO aero_zones(local_id,floor_id,name,zone_type,geom)
		VALUES($1,3,$2,$3,ST_SetSRID(ST_GeomFromGeoJSON($4),0)) RETURNING `+aeroZoneColumns,
		in.LocalID, in.Name, in.ZoneType, string(in.Geometry),
	), &saved)
	if err != nil {
		fail(w, 503, "No se pudo crear la zona")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	jsonResponse(w, saved)
}

func (a *App) patchAeroZone(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, 415, "Se requiere application/json")
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		fail(w, 400, "Id inválido")
		return
	}
	var patch struct {
		Name     *string         `json:"name"`
		ZoneType *string         `json:"zone_type"`
		Geometry json.RawMessage `json:"geometry"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
	d.DisallowUnknownFields()
	if d.Decode(&patch) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400, "Datos inválidos")
		return
	}
	if patch.Name == nil && patch.ZoneType == nil && len(patch.Geometry) == 0 {
		fail(w, 400, "No hay cambios que aplicar")
		return
	}
	if patch.Name != nil {
		n := strings.TrimSpace(*patch.Name)
		if n == "" || len(n) > 100 {
			fail(w, 400, "El nombre debe tener entre 1 y 100 caracteres")
			return
		}
		patch.Name = &n
	}
	if patch.ZoneType != nil && !aeroZoneTypes[*patch.ZoneType] {
		fail(w, 400, "Tipo de zona inválido")
		return
	}
	var geomParam any
	if len(patch.Geometry) > 0 {
		if !validPlanPolygon(patch.Geometry) {
			fail(w, 400, "El polígono debe tener entre 3 y 100 vértices y caer dentro del plano")
			return
		}
		geomParam = string(patch.Geometry)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if geomParam != nil {
		var valid bool
		if err := a.db.QueryRow(ctx, "SELECT ST_IsValid(ST_GeomFromGeoJSON($1))", geomParam).Scan(&valid); err != nil || !valid {
			fail(w, 400, "El polígono se cruza o tiene geometría inválida")
			return
		}
	}
	var saved AeroZone
	err = scanAeroZone(a.db.QueryRow(ctx, `UPDATE aero_zones SET
			name=COALESCE($2,name),
			zone_type=COALESCE($3,zone_type),
			geom=CASE WHEN $4::text IS NULL THEN geom ELSE ST_SetSRID(ST_GeomFromGeoJSON($4),0) END
		WHERE zone_id=$1 AND floor_id=3
		RETURNING `+aeroZoneColumns, id, patch.Name, patch.ZoneType, geomParam,
	), &saved)
	if err == pgx.ErrNoRows {
		fail(w, 404, "Zona no encontrada")
		return
	}
	if err != nil {
		fail(w, 503, "No se pudo guardar la zona")
		return
	}
	jsonResponse(w, saved)
}
