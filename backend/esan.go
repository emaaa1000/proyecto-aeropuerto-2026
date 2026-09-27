package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// esanFloor is the ESAN site (migrations/023_create_esan.sql). Its plan is the
// LAP01 model's floor map, in metres, SRID 0.
const esanFloor = 100

var esanZoneTypes = map[string]bool{"PASILLO": true, "ENTRADA": true, "COLA": true, "CHECKIN": true, "SEGURIDAD": true, "PUERTA": true, "OTRO": true}
var esanGenders = map[string]bool{"HOMBRE": true, "MUJER": true, "SIN_DETERMINAR": true}

// esanCache keeps the replay/insights of finished sessions, which no longer
// change; any import or zone edit clears it.
type esanCache struct {
	mu       sync.Mutex
	entradas map[string][]byte
}

var cacheEsan = &esanCache{entradas: map[string][]byte{}}

func (c *esanCache) get(clave string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entradas[clave]
	return v, ok
}

func (c *esanCache) put(clave string, v []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entradas) >= 64 {
		c.entradas = map[string][]byte{}
	}
	c.entradas[clave] = v
}

func (c *esanCache) limpiar() {
	c.mu.Lock()
	c.entradas = map[string][]byte{}
	c.mu.Unlock()
}

func writeJSONBytes(w http.ResponseWriter, cuerpo []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(cuerpo)
}

// respondCached serialises once and keeps the bytes when the session is finished.
func respondCached(w http.ResponseWriter, clave string, guardar bool, v any) {
	cuerpo, err := json.Marshal(v)
	if err != nil {
		fail(w, http.StatusInternalServerError, "No se pudo preparar la respuesta")
		return
	}
	if guardar {
		cacheEsan.put(clave, cuerpo)
	}
	writeJSONBytes(w, cuerpo)
}

type esanCamera struct {
	ID         string      `json:"camera_id"`
	Name       string      `json:"name"`
	StreamURI  string      `json:"stream_uri"`
	FPS        *float64    `json:"fps"`
	Width      *int        `json:"width_px"`
	Height     *int        `json:"height_px"`
	Offset     float64     `json:"timestamp_offset_s"`
	Position   *[2]float64 `json:"position"`
	Angle      *float64    `json:"angle_deg"`
	Homography []float64   `json:"homography"`
	Active     bool        `json:"active"`
}

type esanZone struct {
	ID     int          `json:"zone_id"`
	Name   string       `json:"name"`
	Type   string       `json:"zone_type"`
	Color  string       `json:"color"`
	Points [][2]float64 `json:"points"`
	AreaM2 float64      `json:"area_m2"`
}

type esanImportCamera struct {
	ID         string      `json:"camera_id"`
	Name       string      `json:"name"`
	FPS        *float64    `json:"fps"`
	Width      *int        `json:"width_px"`
	Height     *int        `json:"height_px"`
	Offset     float64     `json:"timestamp_offset_s"`
	Homography []float64   `json:"homography"`
	Position   *[2]float64 `json:"position"`
	Angle      *float64    `json:"angle_deg"`
}

type esanImportIdentity struct {
	GlobalID   string    `json:"global_id"`
	Number     int       `json:"public_number"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
	NCameras   int       `json:"n_cameras"`
	Gender     string    `json:"gender"`
	GenderConf *float64  `json:"gender_confidence"`
	Votes      int       `json:"gender_votes"`
}

type esanImportTracklet struct {
	ID       string    `json:"tracklet_id"`
	GlobalID string    `json:"global_id"`
	CameraID string    `json:"camera_id"`
	LocalID  int       `json:"local_id"`
	TStart   time.Time `json:"t_start"`
	TEnd     time.Time `json:"t_end"`
	Views    int       `json:"n_reid_views"`
}

// esanImportPoints is columnar (one array per column) to keep the payload small;
// t is seconds since the session's recording_start.
type esanImportPoints struct {
	GlobalID   []string   `json:"global_id"`
	TrackletID []string   `json:"tracklet_id"`
	CameraID   []string   `json:"camera_id"`
	LocalID    []int      `json:"local_id"`
	T          []float64  `json:"t"`
	X          []*float64 `json:"x"`
	Y          []*float64 `json:"y"`
	Speed      []*float64 `json:"speed_mps"`
	Direction  []*float64 `json:"direction_deg"`
	Confidence []float64  `json:"confidence"`
}

type esanImportSession struct {
	ID             string          `json:"session_id"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Status         string          `json:"status"`
	RecordingStart time.Time       `json:"recording_start"`
	EndedAt        *time.Time      `json:"ended_at"`
	ConfigVersion  string          `json:"config_version"`
	ConfigSHA256   string          `json:"config_sha256"`
	Summary        json.RawMessage `json:"summary"`
}

type esanImport struct {
	Session    esanImportSession    `json:"session"`
	Map        json.RawMessage      `json:"map"`
	Cameras    []esanImportCamera   `json:"cameras"`
	Identities []esanImportIdentity `json:"identities"`
	Tracklets  []esanImportTracklet `json:"tracklets"`
	Points     esanImportPoints     `json:"points"`
}

func (a *App) esanRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/esan/config", a.esanConfig)
	m.HandleFunc("PUT /api/v1/esan/cameras/{id}", a.esanSaveCamera)
	m.HandleFunc("POST /api/v1/esan/zones", a.esanSaveZone)
	m.HandleFunc("PUT /api/v1/esan/zones/{id}", a.esanSaveZone)
	m.HandleFunc("DELETE /api/v1/esan/zones/{id}", a.esanDeleteZone)
	m.HandleFunc("GET /api/v1/esan/sessions", a.esanSessions)
	m.HandleFunc("POST /api/v1/esan/sessions", a.esanImport)
	m.HandleFunc("GET /api/v1/esan/sessions/{id}/replay", a.esanReplay)
	m.HandleFunc("GET /api/v1/esan/sessions/{id}/insights", a.esanInsights)
}

func parseUUID(s string) ([16]byte, error) {
	var u [16]byte
	h := strings.ReplaceAll(strings.TrimSpace(s), "-", "")
	if len(h) != 32 {
		return u, fmt.Errorf("UUID inválido: %q", s)
	}
	if _, err := hex.Decode(u[:], []byte(h)); err != nil {
		return u, fmt.Errorf("UUID inválido: %q", s)
	}
	return u, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// esanZoneWKT validates a zone outline (metres) and returns it as a closed WKT polygon.
func esanZoneWKT(points [][2]float64) (string, error) {
	if len(points) < 3 || len(points) > 100 {
		return "", errors.New("la zona necesita entre 3 y 100 vértices")
	}
	var b strings.Builder
	b.WriteString("POLYGON((")
	for i, p := range append(points, points[0]) {
		if !finite(p[0]) || !finite(p[1]) || math.Abs(p[0]) > 1000 || math.Abs(p[1]) > 1000 {
			return "", errors.New("coordenadas de la zona fuera del plano")
		}
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%.4f %.4f", p[0], p[1])
	}
	b.WriteString("))")
	return b.String(), nil
}

// validateEsanImport checks a model session before it touches the database.
func validateEsanImport(p *esanImport) error {
	s := &p.Session
	if _, err := parseUUID(s.ID); err != nil {
		return err
	}
	if s.Kind == "" {
		s.Kind = "BUILD"
	}
	if s.Status == "" {
		s.Status = "DONE"
	}
	if s.Kind != "BUILD" && s.Kind != "LIVE" {
		return errors.New("kind debe ser BUILD o LIVE")
	}
	if s.Status != "RUNNING" && s.Status != "DONE" && s.Status != "FAILED" {
		return errors.New("status debe ser RUNNING, DONE o FAILED")
	}
	if s.RecordingStart.IsZero() || len(s.Name) > 120 || (s.ConfigSHA256 != "" && len(s.ConfigSHA256) != 64) || len(s.ConfigVersion) > 20 {
		return errors.New("datos de sesión inválidos (recording_start, name o config)")
	}
	if s.EndedAt != nil && s.EndedAt.Before(s.RecordingStart) {
		return errors.New("ended_at es anterior a recording_start")
	}
	cameras := map[string]bool{}
	for _, c := range p.Cameras {
		if c.ID == "" || len(c.ID) > 30 || len(c.Name) > 80 || !finite(c.Offset) {
			return fmt.Errorf("cámara inválida: %q", c.ID)
		}
		if c.Homography != nil && len(c.Homography) != 9 {
			return fmt.Errorf("%s: la homografía debe tener 9 valores", c.ID)
		}
		if c.Angle != nil && (!finite(*c.Angle) || *c.Angle < 0 || *c.Angle >= 360) {
			return fmt.Errorf("%s: ángulo fuera de [0, 360)", c.ID)
		}
		cameras[c.ID] = true
	}
	identities := map[string]bool{}
	for i := range p.Identities {
		id := &p.Identities[i]
		if _, err := parseUUID(id.GlobalID); err != nil {
			return err
		}
		id.Gender = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(id.Gender), " ", "_"))
		if id.Gender == "" {
			id.Gender = "SIN_DETERMINAR"
		}
		if !esanGenders[id.Gender] {
			return fmt.Errorf("género inválido: %q", id.Gender)
		}
		if id.Gender != "SIN_DETERMINAR" && id.GenderConf == nil {
			return errors.New("una identidad con género necesita su confianza")
		}
		if id.GenderConf != nil && (*id.GenderConf < 0 || *id.GenderConf > 1) {
			return errors.New("gender_confidence fuera de [0, 1]")
		}
		if id.Number <= 0 || id.NCameras <= 0 || id.NCameras > 32767 || id.Votes < 0 || id.LastSeen.Before(id.FirstSeen) {
			return fmt.Errorf("identidad inválida: %s", id.GlobalID)
		}
		identities[id.GlobalID] = true
	}
	tracklets := map[string]bool{}
	for _, t := range p.Tracklets {
		if _, err := parseUUID(t.ID); err != nil {
			return err
		}
		if !identities[t.GlobalID] || t.CameraID == "" || t.TEnd.Before(t.TStart) || t.Views < 0 {
			return fmt.Errorf("tracklet inválido: %s", t.ID)
		}
		tracklets[t.ID] = true
	}
	pt := &p.Points
	n := len(pt.T)
	for _, l := range []int{len(pt.GlobalID), len(pt.TrackletID), len(pt.CameraID), len(pt.LocalID), len(pt.X), len(pt.Y), len(pt.Speed), len(pt.Direction), len(pt.Confidence)} {
		if l != n {
			return errors.New("las columnas de points deben tener el mismo largo")
		}
	}
	for i := 0; i < n; i++ {
		if !identities[pt.GlobalID[i]] || !tracklets[pt.TrackletID[i]] {
			return fmt.Errorf("punto %d: global_id o tracklet_id desconocido", i)
		}
		if (pt.X[i] == nil) != (pt.Y[i] == nil) || (pt.X[i] != nil && (!finite(*pt.X[i]) || !finite(*pt.Y[i]))) {
			return fmt.Errorf("punto %d: x e y deben venir juntos y ser finitos", i)
		}
		if !finite(pt.T[i]) || pt.T[i] < 0 || pt.Confidence[i] < 0 || pt.Confidence[i] > 1 {
			return fmt.Errorf("punto %d: tiempo o confianza inválidos", i)
		}
		if pt.Speed[i] != nil && (!finite(*pt.Speed[i]) || *pt.Speed[i] < 0) {
			return fmt.Errorf("punto %d: velocidad inválida", i)
		}
		if pt.Direction[i] != nil && (!finite(*pt.Direction[i]) || *pt.Direction[i] < 0 || *pt.Direction[i] >= 360) {
			return fmt.Errorf("punto %d: dirección inválida", i)
		}
	}
	return nil
}

func dbFailure(w http.ResponseWriter, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (strings.HasPrefix(pgErr.Code, "22") || strings.HasPrefix(pgErr.Code, "23")) {
		fail(w, http.StatusBadRequest, "La base rechazó los datos: "+pgErr.Message)
		return
	}
	fail(w, http.StatusInternalServerError, "Error de base de datos")
}

func optionalFloat32(v *float64) *float32 {
	if v == nil {
		return nil
	}
	f := float32(*v)
	return &f
}

// esanImport stores a whole model session (build or live checkpoint) in one
// transaction. Re-sending the same session_id replaces it, so a live session can
// be checkpointed repeatedly with its reconciled identities.
func (a *App) esanImport(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	var p esanImport
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		fail(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if err := validateEsanImport(&p); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "Base de datos no disponible")
		return
	}
	defer tx.Rollback(ctx)

	if len(p.Map) > 0 && string(p.Map) != "null" {
		if _, err = tx.Exec(ctx, `INSERT INTO esan_maps (floor_id, config) VALUES ($1, $2::text::jsonb)
			ON CONFLICT (floor_id) DO UPDATE SET config = excluded.config, updated_at = now()`, esanFloor, string(p.Map)); err != nil {
			dbFailure(w, err)
			return
		}
	}
	for _, c := range p.Cameras {
		var px, py *float64
		if c.Position != nil {
			px, py = &c.Position[0], &c.Position[1]
		}
		name := strings.TrimSpace(c.Name)
		if name == "" {
			name = c.ID
		}
		if _, err = tx.Exec(ctx, `INSERT INTO aero_cameras
			(camera_id, name, floor_id, fps, width_px, height_px, timestamp_offset_s, mode, homography, position, angle_deg)
			VALUES ($1, $2, $3, $4::float8, $5::int, $6::int, $7::float8, 'VISUAL_TEMPORAL', $8::float8[],
			        CASE WHEN $9::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($9::float8, $10::float8), 0) END, $11::float8)
			ON CONFLICT (camera_id) DO UPDATE SET fps = excluded.fps, width_px = excluded.width_px, height_px = excluded.height_px,
			    timestamp_offset_s = excluded.timestamp_offset_s, homography = excluded.homography, position = excluded.position,
			    angle_deg = excluded.angle_deg, floor_id = excluded.floor_id, updated_at = now()`,
			c.ID, name, esanFloor, c.FPS, c.Width, c.Height, c.Offset, c.Homography, px, py, c.Angle); err != nil {
			dbFailure(w, err)
			return
		}
	}

	s := p.Session
	if _, err = tx.Exec(ctx, `DELETE FROM analysis_sessions WHERE session_id = $1::text::uuid`, s.ID); err != nil {
		dbFailure(w, err)
		return
	}
	var summary any
	if len(s.Summary) > 0 && string(s.Summary) != "null" {
		summary = string(s.Summary)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO analysis_sessions
		(session_id, recording_start, started_at, ended_at, mode, config_version, config_sha256, status, floor_id, kind, name, summary)
		VALUES ($1::text::uuid, $2, $2, $3, 'VISUAL_TEMPORAL', NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8, NULLIF($9, ''), $10::text::jsonb)`,
		s.ID, s.RecordingStart, s.EndedAt, s.ConfigVersion, s.ConfigSHA256, s.Status, esanFloor, s.Kind, strings.TrimSpace(s.Name), summary); err != nil {
		dbFailure(w, err)
		return
	}
	sid, _ := parseUUID(s.ID)

	identityRows := make([][]any, 0, len(p.Identities))
	for _, i := range p.Identities {
		g, _ := parseUUID(i.GlobalID)
		identityRows = append(identityRows, []any{g, sid, int32(i.Number), i.FirstSeen, i.LastSeen, int16(i.NCameras), i.Gender, optionalFloat32(i.GenderConf), int32(i.Votes)})
	}
	if _, err = tx.CopyFrom(ctx, pgx.Identifier{"identities"},
		[]string{"global_id", "session_id", "public_number", "first_seen", "last_seen", "n_cameras", "gender_estimate", "gender_confidence", "gender_votes"},
		pgx.CopyFromRows(identityRows)); err != nil {
		dbFailure(w, err)
		return
	}

	trackletRows := make([][]any, 0, len(p.Tracklets))
	for _, t := range p.Tracklets {
		tid, _ := parseUUID(t.ID)
		g, _ := parseUUID(t.GlobalID)
		trackletRows = append(trackletRows, []any{tid, g, t.CameraID, int32(t.LocalID), t.TStart, t.TEnd, int32(t.Views)})
	}
	if _, err = tx.CopyFrom(ctx, pgx.Identifier{"tracklets"},
		[]string{"tracklet_id", "global_id", "camera_id", "local_id", "t_start", "t_end", "n_reid_views"},
		pgx.CopyFromRows(trackletRows)); err != nil {
		dbFailure(w, err)
		return
	}

	pt := p.Points
	pointRows := make([][]any, 0, len(pt.T))
	for i := range pt.T {
		g, _ := parseUUID(pt.GlobalID[i])
		tid, _ := parseUUID(pt.TrackletID[i])
		at := s.RecordingStart.Add(time.Duration(pt.T[i] * float64(time.Second)))
		pointRows = append(pointRows, []any{g, tid, pt.CameraID[i], int32(pt.LocalID[i]), at, pt.X[i], pt.Y[i],
			optionalFloat32(pt.Speed[i]), optionalFloat32(pt.Direction[i]), float32(pt.Confidence[i])})
	}
	if _, err = tx.CopyFrom(ctx, pgx.Identifier{"trajectory_points"},
		[]string{"global_id", "tracklet_id", "camera_id", "local_id", "timestamp", "x", "y", "speed_mps", "direction_deg", "confidence"},
		pgx.CopyFromRows(pointRows)); err != nil {
		dbFailure(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		dbFailure(w, err)
		return
	}
	cacheEsan.limpiar()
	jsonResponse(w, map[string]any{"session_id": s.ID, "identities": len(identityRows), "tracklets": len(trackletRows),
		"points": len(pointRows), "ms": time.Since(start).Milliseconds()})
}

func (a *App) esanConfig(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var mapa json.RawMessage
	var raw []byte
	err := a.db.QueryRow(ctx, "SELECT config::text FROM esan_maps WHERE floor_id = $1", esanFloor).Scan(&raw)
	if err == nil {
		mapa = raw
	} else if !errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusInternalServerError, "Error de base de datos")
		return
	}
	rows, err := a.db.Query(ctx, `SELECT camera_id, name, COALESCE(stream_uri, ''), fps::float8, width_px, height_px,
		timestamp_offset_s::float8, ST_X(position), ST_Y(position), angle_deg::float8, homography, active
		FROM aero_cameras WHERE floor_id = $1 ORDER BY camera_id`, esanFloor)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Error de base de datos")
		return
	}
	cameras := []esanCamera{}
	for rows.Next() {
		var c esanCamera
		var x, y *float64
		if err = rows.Scan(&c.ID, &c.Name, &c.StreamURI, &c.FPS, &c.Width, &c.Height, &c.Offset, &x, &y, &c.Angle, &c.Homography, &c.Active); err != nil {
			rows.Close()
			fail(w, http.StatusInternalServerError, "Error de base de datos")
			return
		}
		if x != nil && y != nil {
			c.Position = &[2]float64{*x, *y}
		}
		cameras = append(cameras, c)
	}
	rows.Close()
	zones, err := a.esanZones(ctx)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Error de base de datos")
		return
	}
	jsonResponse(w, map[string]any{"floor_id": esanFloor, "map": mapa, "cameras": cameras, "zones": zones})
}

func (a *App) esanZones(ctx context.Context) ([]esanZone, error) {
	rows, err := a.db.Query(ctx, `SELECT zone_id, name, zone_type, COALESCE(color, ''), ST_AsGeoJSON(geom), area_m2
		FROM aero_zones WHERE floor_id = $1 AND active ORDER BY zone_id`, esanFloor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	zones := []esanZone{}
	for rows.Next() {
		var z esanZone
		var geo string
		if err = rows.Scan(&z.ID, &z.Name, &z.Type, &z.Color, &geo, &z.AreaM2); err != nil {
			return nil, err
		}
		var g struct {
			Coordinates [][][2]float64 `json:"coordinates"`
		}
		if json.Unmarshal([]byte(geo), &g) == nil && len(g.Coordinates) > 0 {
			ring := g.Coordinates[0]
			if len(ring) > 1 {
				ring = ring[:len(ring)-1]
			}
			z.Points = ring
		}
		z.AreaM2 = round2(z.AreaM2)
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

func (a *App) esanSaveCamera(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	var in struct {
		Name      string `json:"name"`
		StreamURI string `json:"stream_uri"`
		Active    *bool  `json:"active"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil {
		fail(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	in.Name, in.StreamURI = strings.TrimSpace(in.Name), strings.TrimSpace(in.StreamURI)
	if in.Name == "" || len(in.Name) > 80 || len(in.StreamURI) > 500 {
		fail(w, http.StatusBadRequest, "Nombre (1-80) o fuente (hasta 500) inválidos")
		return
	}
	if in.StreamURI != "" {
		u, err := url.Parse(in.StreamURI)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "rtsp" && u.Scheme != "rtsps") {
			fail(w, http.StatusBadRequest, "La fuente debe ser una URL http(s):// o rtsp:// con host")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tag, err := a.db.Exec(ctx, `UPDATE aero_cameras SET name = $2, stream_uri = NULLIF($3, ''), active = COALESCE($4, active), updated_at = now()
		WHERE camera_id = $1 AND floor_id = $5`, r.PathValue("id"), in.Name, in.StreamURI, in.Active, esanFloor)
	if err != nil {
		dbFailure(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "Cámara no encontrada en ESAN")
		return
	}
	jsonResponse(w, map[string]any{"camera_id": r.PathValue("id"), "name": in.Name, "stream_uri": in.StreamURI})
}

func (a *App) esanSaveZone(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	var in struct {
		Name   string       `json:"name"`
		Type   string       `json:"zone_type"`
		Color  string       `json:"color"`
		Points [][2]float64 `json:"points"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil {
		fail(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 100 || !esanZoneTypes[in.Type] || (in.Color != "" && !colorPattern.MatchString(in.Color)) {
		fail(w, http.StatusBadRequest, "Nombre, tipo o color de zona inválidos")
		return
	}
	wkt, err := esanZoneWKT(in.Points)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var z esanZone
	if raw := r.PathValue("id"); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil {
			fail(w, http.StatusBadRequest, "Zona inválida")
			return
		}
		err = a.db.QueryRow(ctx, `UPDATE aero_zones SET name = $2, zone_type = $3, color = NULLIF($4, ''), geom = ST_GeomFromText($5, 0)
			WHERE zone_id = $1 AND floor_id = $6 RETURNING zone_id, area_m2`, id, in.Name, in.Type, in.Color, wkt, esanFloor).Scan(&z.ID, &z.AreaM2)
	} else {
		err = a.db.QueryRow(ctx, `INSERT INTO aero_zones (floor_id, name, zone_type, color, geom)
			VALUES ($1, $2, $3, NULLIF($4, ''), ST_GeomFromText($5, 0)) RETURNING zone_id, area_m2`,
			esanFloor, in.Name, in.Type, in.Color, wkt).Scan(&z.ID, &z.AreaM2)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "Zona no encontrada en ESAN")
		return
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			fail(w, http.StatusBadRequest, "El polígono no es válido: sus lados se cruzan")
			return
		}
		dbFailure(w, err)
		return
	}
	z.Name, z.Type, z.Color, z.Points, z.AreaM2 = in.Name, in.Type, in.Color, in.Points, round2(z.AreaM2)
	cacheEsan.limpiar()
	jsonResponse(w, z)
}

func (a *App) esanDeleteZone(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "Zona inválida")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tag, err := a.db.Exec(ctx, "DELETE FROM aero_zones WHERE zone_id = $1 AND floor_id = $2", id, esanFloor)
	if err != nil {
		dbFailure(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "Zona no encontrada en ESAN")
		return
	}
	cacheEsan.limpiar()
	w.WriteHeader(http.StatusNoContent)
}

type esanSession struct {
	ID             string          `json:"session_id"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Status         string          `json:"status"`
	RecordingStart time.Time       `json:"recording_start"`
	EndedAt        *time.Time      `json:"ended_at"`
	Summary        json.RawMessage `json:"summary"`
	Identities     int             `json:"identities"`
	Points         int             `json:"points"`
	DurationS      float64         `json:"duration_s"`
}

func (a *App) esanSessions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := a.db.Query(ctx, `SELECT s.session_id::text, COALESCE(s.name, ''), s.kind, s.status, s.recording_start, s.ended_at,
		s.summary::text, (SELECT count(*) FROM identities i WHERE i.session_id = s.session_id), COALESCE(p.n, 0), COALESCE(p.dur, 0)
		FROM analysis_sessions s
		LEFT JOIN LATERAL (SELECT count(*) AS n, EXTRACT(EPOCH FROM max(tp."timestamp") - s.recording_start)::float8 AS dur
		                   FROM trajectory_points tp JOIN identities i ON i.global_id = tp.global_id
		                   WHERE i.session_id = s.session_id) p ON TRUE
		WHERE s.floor_id = $1 ORDER BY s.recording_start DESC LIMIT 50`, esanFloor)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Error de base de datos")
		return
	}
	defer rows.Close()
	sessions := []esanSession{}
	for rows.Next() {
		var s esanSession
		var summary *string
		if err = rows.Scan(&s.ID, &s.Name, &s.Kind, &s.Status, &s.RecordingStart, &s.EndedAt, &summary, &s.Identities, &s.Points, &s.DurationS); err != nil {
			fail(w, http.StatusInternalServerError, "Error de base de datos")
			return
		}
		if summary != nil {
			s.Summary = json.RawMessage(*summary)
		}
		s.DurationS = round2(s.DurationS)
		sessions = append(sessions, s)
	}
	jsonResponse(w, sessions)
}

func (a *App) esanSession(ctx context.Context, w http.ResponseWriter, raw string) (string, esanSession, bool) {
	var s esanSession
	if _, err := parseUUID(raw); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return "", s, false
	}
	var summary *string
	err := a.db.QueryRow(ctx, `SELECT session_id::text, COALESCE(name, ''), kind, status, recording_start, ended_at, summary::text
		FROM analysis_sessions WHERE session_id = $1::text::uuid AND floor_id = $2`, raw, esanFloor).
		Scan(&s.ID, &s.Name, &s.Kind, &s.Status, &s.RecordingStart, &s.EndedAt, &summary)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "Sesión no encontrada")
		return "", s, false
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "Error de base de datos")
		return "", s, false
	}
	if summary != nil {
		s.Summary = json.RawMessage(*summary)
	}
	return s.ID, s, true
}

type esanTrack struct {
	Number     int      `json:"numero"`
	Gender     string   `json:"genero"`
	Confidence *float64 `json:"confianza"`
	Cameras    []string `json:"camaras"`
	FirstS     float64  `json:"inicio_s"`
	LastS      float64  `json:"fin_s"`
	K          []int    `json:"k"`
	X          []float64 `json:"x"`
	Y          []float64 `json:"y"`
}

// esanReplay returns one map position per person every `paso` seconds (the
// average of every camera that saw them), ready to animate in the browser.
func (a *App) esanReplay(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, session, ok := a.esanSession(ctx, w, r.PathValue("id"))
	if !ok {
		return
	}
	step := 0.2
	if raw := r.URL.Query().Get("paso"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v < 0.05 || v > 5 {
			fail(w, http.StatusBadRequest, "paso debe estar entre 0.05 y 5 segundos")
			return
		}
		step = v
	}
	clave := fmt.Sprintf("replay:%s:%g", id, step)
	if cuerpo, hit := cacheEsan.get(clave); hit && session.Status == "DONE" {
		writeJSONBytes(w, cuerpo)
		return
	}
	rows, err := a.db.Query(ctx, `SELECT i.public_number, i.gender_estimate, i.gender_confidence::float8,
		EXTRACT(EPOCH FROM i.first_seen - s.recording_start)::float8, EXTRACT(EPOCH FROM i.last_seen - s.recording_start)::float8,
		COALESCE((SELECT array_agg(DISTINCT t.camera_id ORDER BY t.camera_id) FROM tracklets t WHERE t.global_id = i.global_id), '{}')
		FROM identities i JOIN analysis_sessions s ON s.session_id = i.session_id
		WHERE i.session_id = $1::text::uuid ORDER BY i.public_number`, id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Error de base de datos")
		return
	}
	people := map[int]*esanTrack{}
	order := []int{}
	for rows.Next() {
		t := &esanTrack{K: []int{}, X: []float64{}, Y: []float64{}}
		if err = rows.Scan(&t.Number, &t.Gender, &t.Confidence, &t.FirstS, &t.LastS, &t.Cameras); err != nil {
			rows.Close()
			fail(w, http.StatusInternalServerError, "Error de base de datos")
			return
		}
		t.FirstS, t.LastS = round2(t.FirstS), round2(t.LastS)
		people[t.Number] = t
		order = append(order, t.Number)
	}
	rows.Close()
	rows, err = a.db.Query(ctx, `SELECT i.public_number, floor(EXTRACT(EPOCH FROM tp."timestamp" - s.recording_start) / $2)::int AS k,
		avg(tp.x), avg(tp.y)
		FROM trajectory_points tp
		JOIN identities i ON i.global_id = tp.global_id
		JOIN analysis_sessions s ON s.session_id = i.session_id
		WHERE i.session_id = $1::text::uuid AND tp.x IS NOT NULL
		GROUP BY 1, 2 ORDER BY 1, 2`, id, step)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Error de base de datos")
		return
	}
	defer rows.Close()
	maxK := 0
	for rows.Next() {
		var n, k int
		var x, y float64
		if err = rows.Scan(&n, &k, &x, &y); err != nil {
			fail(w, http.StatusInternalServerError, "Error de base de datos")
			return
		}
		if t := people[n]; t != nil {
			t.K, t.X, t.Y = append(t.K, k), append(t.X, round2(x)), append(t.Y, round2(y))
		}
		if k > maxK {
			maxK = k
		}
	}
	tracks := make([]*esanTrack, 0, len(order))
	duration := float64(maxK) * step
	for _, n := range order {
		tracks = append(tracks, people[n])
		if people[n].LastS > duration {
			duration = people[n].LastS
		}
	}
	respondCached(w, clave, session.Status == "DONE", map[string]any{"session": session, "paso_s": step, "duracion_s": round2(duration), "personas": tracks})
}

// esanInsights aggregates a session with SQL: people, gender, dwell, occupancy
// over time, cameras, flows between cameras, a 1 m heat map and zones.
func (a *App) esanInsights(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	id, session, ok := a.esanSession(ctx, w, r.PathValue("id"))
	if !ok {
		return
	}
	clave := "insights:" + id
	if cuerpo, hit := cacheEsan.get(clave); hit && session.Status == "DONE" {
		writeJSONBytes(w, cuerpo)
		return
	}
	failDB := func() { fail(w, http.StatusInternalServerError, "Error de base de datos") }

	type person struct {
		Number     int      `json:"numero"`
		Gender     string   `json:"genero"`
		Confidence *float64 `json:"confianza"`
		Cameras    int      `json:"camaras"`
		DwellS     float64  `json:"permanencia_s"`
		FirstS     float64  `json:"inicio_s"`
	}
	people := []person{}
	gender := map[string]int{"HOMBRE": 0, "MUJER": 0, "SIN_DETERMINAR": 0}
	rows, err := a.db.Query(ctx, `SELECT i.public_number, i.gender_estimate, i.gender_confidence::float8, i.n_cameras,
		EXTRACT(EPOCH FROM i.last_seen - i.first_seen)::float8, EXTRACT(EPOCH FROM i.first_seen - s.recording_start)::float8
		FROM identities i JOIN analysis_sessions s ON s.session_id = i.session_id
		WHERE i.session_id = $1::text::uuid ORDER BY i.public_number`, id)
	if err != nil {
		failDB()
		return
	}
	for rows.Next() {
		var p person
		if err = rows.Scan(&p.Number, &p.Gender, &p.Confidence, &p.Cameras, &p.DwellS, &p.FirstS); err != nil {
			rows.Close()
			failDB()
			return
		}
		p.DwellS, p.FirstS = round2(p.DwellS), round2(p.FirstS)
		gender[p.Gender]++
		people = append(people, p)
	}
	rows.Close()
	multicam := 0
	dwell := make([]float64, 0, len(people))
	for _, p := range people {
		if p.Cameras > 1 {
			multicam++
		}
		dwell = append(dwell, p.DwellS)
	}
	sort.Float64s(dwell)
	dwellStats := map[string]float64{"media_s": 0, "mediana_s": 0, "max_s": 0}
	if len(dwell) > 0 {
		total := 0.0
		for _, v := range dwell {
			total += v
		}
		dwellStats["media_s"] = round2(total / float64(len(dwell)))
		dwellStats["mediana_s"] = dwell[len(dwell)/2]
		dwellStats["max_s"] = dwell[len(dwell)-1]
	}

	const base = `FROM trajectory_points tp JOIN identities i ON i.global_id = tp.global_id
		JOIN analysis_sessions s ON s.session_id = i.session_id WHERE i.session_id = $1::text::uuid`
	var points int
	var duration, speedMean, speedMedian, stoppedPct *float64
	if err = a.db.QueryRow(ctx, `SELECT count(*), EXTRACT(EPOCH FROM max(tp."timestamp") - min(s.recording_start))::float8,
		avg(tp.speed_mps) FILTER (WHERE tp.speed_mps BETWEEN 0.2 AND 4)::float8,
		(percentile_cont(0.5) WITHIN GROUP (ORDER BY tp.speed_mps) FILTER (WHERE tp.speed_mps BETWEEN 0.2 AND 4))::float8,
		(100.0 * avg((tp.speed_mps < 0.2)::int) FILTER (WHERE tp.speed_mps IS NOT NULL))::float8 `+base, id).
		Scan(&points, &duration, &speedMean, &speedMedian, &stoppedPct); err != nil {
		failDB()
		return
	}
	roundPtr := func(v *float64) *float64 {
		if v == nil {
			return nil
		}
		x := round2(*v)
		return &x
	}

	occupancySeconds, occupancyPeople := []int{}, []int{}
	rows, err = a.db.Query(ctx, `SELECT floor(EXTRACT(EPOCH FROM tp."timestamp" - s.recording_start))::int AS seg,
		count(DISTINCT tp.global_id) `+base+` GROUP BY 1 ORDER BY 1`, id)
	if err != nil {
		failDB()
		return
	}
	for rows.Next() {
		var sec, n int
		if err = rows.Scan(&sec, &n); err != nil {
			rows.Close()
			failDB()
			return
		}
		occupancySeconds, occupancyPeople = append(occupancySeconds, sec), append(occupancyPeople, n)
	}
	rows.Close()

	type cameraStat struct {
		ID     string `json:"camera_id"`
		People int    `json:"personas"`
		Points int    `json:"puntos"`
	}
	cameras := []cameraStat{}
	rows, err = a.db.Query(ctx, `SELECT tp.camera_id, count(DISTINCT tp.global_id), count(*) `+base+` GROUP BY 1 ORDER BY 1`, id)
	if err != nil {
		failDB()
		return
	}
	for rows.Next() {
		var c cameraStat
		if err = rows.Scan(&c.ID, &c.People, &c.Points); err != nil {
			rows.Close()
			failDB()
			return
		}
		cameras = append(cameras, c)
	}
	rows.Close()

	type flow struct {
		From   string `json:"desde"`
		To     string `json:"hacia"`
		People int    `json:"personas"`
	}
	flows := []flow{}
	rows, err = a.db.Query(ctx, `SELECT prev, camera_id, count(DISTINCT global_id) FROM (
		SELECT t.global_id, t.camera_id, lag(t.camera_id) OVER (PARTITION BY t.global_id ORDER BY t.t_start) AS prev
		FROM tracklets t JOIN identities i ON i.global_id = t.global_id WHERE i.session_id = $1::text::uuid) q
		WHERE prev IS NOT NULL AND prev <> camera_id GROUP BY 1, 2 ORDER BY 3 DESC, 1, 2`, id)
	if err != nil {
		failDB()
		return
	}
	for rows.Next() {
		var f flow
		if err = rows.Scan(&f.From, &f.To, &f.People); err != nil {
			rows.Close()
			failDB()
			return
		}
		flows = append(flows, f)
	}
	rows.Close()

	heat := [][3]float64{}
	rows, err = a.db.Query(ctx, `SELECT cx, cy, count(*) FROM (
		SELECT DISTINCT tp.global_id, floor(EXTRACT(EPOCH FROM tp."timestamp" - s.recording_start))::int AS seg,
		       floor(tp.x)::int AS cx, floor(tp.y)::int AS cy `+base+` AND tp.x IS NOT NULL) q GROUP BY 1, 2`, id)
	if err != nil {
		failDB()
		return
	}
	for rows.Next() {
		var cx, cy, n int
		if err = rows.Scan(&cx, &cy, &n); err != nil {
			rows.Close()
			failDB()
			return
		}
		heat = append(heat, [3]float64{float64(cx), float64(cy), float64(n)})
	}
	rows.Close()

	type zoneStat struct {
		ID       int     `json:"zone_id"`
		Name     string  `json:"name"`
		Type     string  `json:"zone_type"`
		AreaM2   float64 `json:"area_m2"`
		Visitors int     `json:"visitantes"`
		Seconds  int     `json:"segundos_persona"`
		DwellS   float64 `json:"permanencia_media_s"`
	}
	zones := []zoneStat{}
	rows, err = a.db.Query(ctx, `WITH d AS (
		SELECT DISTINCT ON (tp.global_id, floor(EXTRACT(EPOCH FROM tp."timestamp" - s.recording_start)))
		       tp.global_id, tp.geom `+base+` AND tp.geom IS NOT NULL
		ORDER BY tp.global_id, floor(EXTRACT(EPOCH FROM tp."timestamp" - s.recording_start)), tp."timestamp")
		SELECT z.zone_id, z.name, z.zone_type, z.area_m2, count(DISTINCT d.global_id), count(d.global_id)
		FROM aero_zones z LEFT JOIN d ON ST_Covers(z.geom, d.geom)
		WHERE z.floor_id = $2 AND z.active GROUP BY z.zone_id ORDER BY z.zone_id`, id, esanFloor)
	if err != nil {
		failDB()
		return
	}
	for rows.Next() {
		var z zoneStat
		if err = rows.Scan(&z.ID, &z.Name, &z.Type, &z.AreaM2, &z.Visitors, &z.Seconds); err != nil {
			rows.Close()
			failDB()
			return
		}
		z.AreaM2 = round2(z.AreaM2)
		if z.Visitors > 0 {
			z.DwellS = round2(float64(z.Seconds) / float64(z.Visitors))
		}
		zones = append(zones, z)
	}
	rows.Close()

	respondCached(w, clave, session.Status == "DONE", map[string]any{
		"session": session,
		"resumen": map[string]any{
			"personas": len(people), "multicamara": multicam, "puntos": points, "duracion_s": roundPtr(duration),
			"genero": gender, "permanencia": dwellStats,
			"velocidad": map[string]any{"media_mps": roundPtr(speedMean), "mediana_mps": roundPtr(speedMedian), "detenidos_pct": roundPtr(stoppedPct)},
		},
		"personas":  people,
		"ocupacion": map[string]any{"segundos": occupancySeconds, "personas": occupancyPeople},
		"camaras":   cameras,
		"flujos":    flows,
		"calor":     map[string]any{"celda_m": 1, "celdas": heat},
		"zonas":     zones,
	})
}
