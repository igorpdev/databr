package ambiental_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/ambiental"
)

// fakeDETERResponse is a minimal GeoJSON FeatureCollection matching the INPE DETER WFS format.
var fakeDETERResponse = map[string]any{
	"type":           "FeatureCollection",
	"totalFeatures":  2,
	"numberMatched":  2,
	"numberReturned": 2,
	"features": []map[string]any{
		{
			"type": "Feature",
			"id":   "deter_amz.fid-abc123",
			"geometry": map[string]any{
				"type":        "MultiPolygon",
				"coordinates": []any{},
			},
			"properties": map[string]any{
				"gid":          "100003_hist",
				"classname":    "DESMATAMENTO_CR",
				"areamunkm":    0.1017,
				"municipality": "obidos",
				"uf":           "PA",
				"view_date":    "2024-01-14",
				"sensor":       "AWFI",
				"satellite":    "CBERS-4",
			},
		},
		{
			"type": "Feature",
			"id":   "deter_amz.fid-def456",
			"geometry": map[string]any{
				"type":        "MultiPolygon",
				"coordinates": []any{},
			},
			"properties": map[string]any{
				"gid":          "100004_hist",
				"classname":    "DEGRADACAO",
				"areamunkm":    0.2500,
				"municipality": "Porto de Moz",
				"uf":           "PA",
				"view_date":    "2024-01-15",
				"sensor":       "WFI",
				"satellite":    "AMAZONIA-1",
			},
		},
	},
}

// fakePRODESResponse is a minimal GeoJSON FeatureCollection matching PRODES yearly_deforestation WFS format.
var fakePRODESResponse = map[string]any{
	"type":           "FeatureCollection",
	"totalFeatures":  3,
	"numberMatched":  3,
	"numberReturned": 3,
	"features": []map[string]any{
		{
			"type": "Feature",
			"id":   "yearly_deforestation.uuid-001",
			"properties": map[string]any{
				"uid":          1,
				"state":        "PA",
				"main_class":   "desmatamento",
				"class_name":   "d2023",
				"year":         2023,
				"area_km":      1.245,
				"publish_year": "2023-01-01",
			},
		},
		{
			"type": "Feature",
			"id":   "yearly_deforestation.uuid-002",
			"properties": map[string]any{
				"uid":          2,
				"state":        "AM",
				"main_class":   "desmatamento",
				"class_name":   "d2023",
				"year":         2023,
				"area_km":      0.987,
				"publish_year": "2023-01-01",
			},
		},
		{
			"type": "Feature",
			"id":   "yearly_deforestation.uuid-003",
			"properties": map[string]any{
				"uid":          3,
				"state":        "MT",
				"main_class":   "desmatamento",
				"class_name":   "d2022",
				"year":         2022,
				"area_km":      2.001,
				"publish_year": "2022-01-01",
			},
		},
	},
}

func newJSONServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// --- DETER Collector tests ---

func TestDETERCollector_Source(t *testing.T) {
	srv := newJSONServer(t, fakeDETERResponse)
	defer srv.Close()
	c := ambiental.NewDETERCollector(srv.URL)
	if got := c.Source(); got != "inpe_deter" {
		t.Errorf("Source() = %q, want %q", got, "inpe_deter")
	}
}

func TestDETERCollector_Schedule(t *testing.T) {
	srv := newJSONServer(t, fakeDETERResponse)
	defer srv.Close()
	c := ambiental.NewDETERCollector(srv.URL)
	if got := c.Schedule(); got != "0 15 * * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 15 * * *")
	}
}

func TestDETERCollector_Collect(t *testing.T) {
	srv := newJSONServer(t, fakeDETERResponse)
	defer srv.Close()

	c := ambiental.NewDETERCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Collect() returned 0 records")
	}

	r := records[0]
	if r.Source != "inpe_deter" {
		t.Errorf("Source = %q, want inpe_deter", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["area_km2"]; !ok {
		t.Error("Data must contain 'area_km2' field")
	}
	if _, ok := r.Data["municipio"]; !ok {
		t.Error("Data must contain 'municipio' field")
	}
	if _, ok := r.Data["estado"]; !ok {
		t.Error("Data must contain 'estado' field")
	}
	if _, ok := r.Data["data_deteccao"]; !ok {
		t.Error("Data must contain 'data_deteccao' field")
	}
	if _, ok := r.Data["classe_desmatamento"]; !ok {
		t.Error("Data must contain 'classe_desmatamento' field")
	}
}

func TestDETERCollector_EmptyResponse(t *testing.T) {
	emptyResp := map[string]any{
		"type":           "FeatureCollection",
		"totalFeatures":  0,
		"numberReturned": 0,
		"features":       []any{},
	}
	srv := newJSONServer(t, emptyResp)
	defer srv.Close()

	c := ambiental.NewDETERCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

// --- PRODES Collector tests ---

func TestPRODESCollector_Source(t *testing.T) {
	srv := newJSONServer(t, fakePRODESResponse)
	defer srv.Close()
	c := ambiental.NewPRODESCollector(srv.URL)
	if got := c.Source(); got != "inpe_prodes" {
		t.Errorf("Source() = %q, want %q", got, "inpe_prodes")
	}
}

func TestPRODESCollector_Schedule(t *testing.T) {
	srv := newJSONServer(t, fakePRODESResponse)
	defer srv.Close()
	c := ambiental.NewPRODESCollector(srv.URL)
	if got := c.Schedule(); got != "@monthly" {
		t.Errorf("Schedule() = %q, want @monthly", got)
	}
}

func TestPRODESCollector_Collect(t *testing.T) {
	srv := newJSONServer(t, fakePRODESResponse)
	defer srv.Close()

	c := ambiental.NewPRODESCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Collect() returned 0 records")
	}

	r := records[0]
	if r.Source != "inpe_prodes" {
		t.Errorf("Source = %q, want inpe_prodes", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["area_km2"]; !ok {
		t.Error("Data must contain 'area_km2' field")
	}
	if _, ok := r.Data["ano"]; !ok {
		t.Error("Data must contain 'ano' field")
	}
	if _, ok := r.Data["estado"]; !ok {
		t.Error("Data must contain 'estado' field")
	}
}

func TestPRODESCollector_EmptyResponse(t *testing.T) {
	emptyResp := map[string]any{
		"type":           "FeatureCollection",
		"totalFeatures":  0,
		"numberReturned": 0,
		"features":       []any{},
	}
	srv := newJSONServer(t, emptyResp)
	defer srv.Close()

	c := ambiental.NewPRODESCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}
