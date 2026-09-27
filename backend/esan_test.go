package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseUUID(t *testing.T) {
	u, err := parseUUID("9be42dae-1b28-5029-a5dc-ae96ae4c2a1e")
	if err != nil || u[0] != 0x9b || u[15] != 0x1e {
		t.Fatalf("uuid válido rechazado: %v %x", err, u)
	}
	for _, bad := range []string{"", "no-es-uuid", "9be42dae-1b28-5029-a5dc-ae96ae4c2a1", "zze42dae1b285029a5dcae96ae4c2a1e"} {
		if _, err := parseUUID(bad); err == nil {
			t.Fatalf("uuid inválido aceptado: %q", bad)
		}
	}
}

func TestEsanZoneWKT(t *testing.T) {
	wkt, err := esanZoneWKT([][2]float64{{0, 0}, {4, 0}, {4, 3}})
	if err != nil || wkt != "POLYGON((0.0000 0.0000,4.0000 0.0000,4.0000 3.0000,0.0000 0.0000))" {
		t.Fatalf("WKT inesperado: %v %s", err, wkt)
	}
	if _, err := esanZoneWKT([][2]float64{{0, 0}, {1, 1}}); err == nil {
		t.Fatal("zona de 2 vértices aceptada")
	}
	if _, err := esanZoneWKT([][2]float64{{0, 0}, {5000, 0}, {1, 1}}); err == nil {
		t.Fatal("zona fuera del plano aceptada")
	}
}

func payloadValido() esanImport {
	inicio := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	x, y, conf := 1.5, 2.5, 0.8
	var p esanImport
	body := `{"session":{"session_id":"11111111-1111-1111-1111-111111111111","name":"prueba","recording_start":"2026-09-26T12:00:00Z"},
	"identities":[{"global_id":"22222222-2222-2222-2222-222222222222","public_number":1,"first_seen":"2026-09-26T12:00:00Z","last_seen":"2026-09-26T12:00:05Z","n_cameras":1,"gender":"Hombre","gender_confidence":0.8,"gender_votes":5}],
	"tracklets":[{"tracklet_id":"33333333-3333-3333-3333-333333333333","global_id":"22222222-2222-2222-2222-222222222222","camera_id":"cam01","local_id":1,"t_start":"2026-09-26T12:00:00Z","t_end":"2026-09-26T12:00:05Z","n_reid_views":3}]}`
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		panic(err)
	}
	p.Points = esanImportPoints{
		GlobalID: []string{"22222222-2222-2222-2222-222222222222"}, TrackletID: []string{"33333333-3333-3333-3333-333333333333"},
		CameraID: []string{"cam01"}, LocalID: []int{1}, T: []float64{0.5}, X: []*float64{&x}, Y: []*float64{&y},
		Speed: []*float64{nil}, Direction: []*float64{nil}, Confidence: []float64{conf},
	}
	_ = inicio
	return p
}

func TestValidateEsanImport(t *testing.T) {
	p := payloadValido()
	if err := validateEsanImport(&p); err != nil {
		t.Fatalf("payload válido rechazado: %v", err)
	}
	if p.Session.Kind != "BUILD" || p.Session.Status != "DONE" || p.Identities[0].Gender != "HOMBRE" {
		t.Fatalf("valores por defecto o género no normalizados: %+v", p.Session)
	}

	casos := map[string]func(*esanImport){
		"columnas desiguales": func(p *esanImport) { p.Points.LocalID = append(p.Points.LocalID, 2) },
		"tracklet desconocido": func(p *esanImport) { p.Points.TrackletID[0] = "44444444-4444-4444-4444-444444444444" },
		"x sin y":             func(p *esanImport) { p.Points.Y[0] = nil },
		"genero sin confianza": func(p *esanImport) { p.Identities[0].GenderConf = nil },
		"genero raro":          func(p *esanImport) { p.Identities[0].Gender = "otro" },
		"kind raro":            func(p *esanImport) { p.Session.Kind = "OTRO" },
		"homografia corta":     func(p *esanImport) { p.Cameras = []esanImportCamera{{ID: "cam01", Homography: []float64{1, 2}}} },
	}
	for nombre, romper := range casos {
		p := payloadValido()
		romper(&p)
		if err := validateEsanImport(&p); err == nil || strings.TrimSpace(err.Error()) == "" {
			t.Fatalf("%s: se aceptó un payload inválido", nombre)
		}
	}
}
