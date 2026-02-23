package ambiental_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/ambiental"
)

var fakeMapBiomasResponse = map[string]any{
	"data": []map[string]any{
		{
			"territory_id":   1,
			"territory_name": "Brasil",
			"year":           2025,
			"class_id":       3,
			"class_name":     "Formacao Florestal",
			"area_ha":        450000000.5,
			"percentage":     52.8,
		},
		{
			"territory_id":   1,
			"territory_name": "Brasil",
			"year":           2025,
			"class_id":       15,
			"class_name":     "Pastagem",
			"area_ha":        180000000.2,
			"percentage":     21.1,
		},
		{
			"territory_id":   1,
			"territory_name": "Brasil",
			"year":           2025,
			"class_id":       39,
			"class_name":     "Soja",
			"area_ha":        40000000.0,
			"percentage":     4.7,
		},
	},
}

func newMapBiomasServer(t *testing.T, resp any, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestMapBiomasCollector_Source(t *testing.T) {
	c := ambiental.NewMapBiomasCollector("http://localhost")
	if got := c.Source(); got != "mapbiomas_cobertura" {
		t.Errorf("Source() = %q, want %q", got, "mapbiomas_cobertura")
	}
}

func TestMapBiomasCollector_Schedule(t *testing.T) {
	c := ambiental.NewMapBiomasCollector("http://localhost")
	if got := c.Schedule(); got != "0 8 1 * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 8 1 * *")
	}
}

func TestMapBiomasCollector_Collect(t *testing.T) {
	srv := newMapBiomasServer(t, fakeMapBiomasResponse, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewMapBiomasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
}

func TestMapBiomasCollector_Collect_RecordKey(t *testing.T) {
	srv := newMapBiomasServer(t, fakeMapBiomasResponse, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewMapBiomasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// RecordKey format: coverage_YEAR_TERRITORYID_CLASSID
	if records[0].RecordKey != "coverage_2025_1_3" {
		t.Errorf("records[0].RecordKey = %q, want %q", records[0].RecordKey, "coverage_2025_1_3")
	}
	if records[1].RecordKey != "coverage_2025_1_15" {
		t.Errorf("records[1].RecordKey = %q, want %q", records[1].RecordKey, "coverage_2025_1_15")
	}
}

func TestMapBiomasCollector_Collect_SourceField(t *testing.T) {
	srv := newMapBiomasServer(t, fakeMapBiomasResponse, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewMapBiomasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.Source != "mapbiomas_cobertura" {
			t.Errorf("Source = %q, want %q", rec.Source, "mapbiomas_cobertura")
		}
	}
}

func TestMapBiomasCollector_Collect_DataFields(t *testing.T) {
	srv := newMapBiomasServer(t, fakeMapBiomasResponse, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewMapBiomasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one record")
	}

	rec := records[0]
	required := []string{
		"territory_id", "territory_name", "year",
		"class_id", "class_name", "area_ha", "percentage",
	}
	for _, field := range required {
		if _, ok := rec.Data[field]; !ok {
			t.Errorf("Data must contain field %q", field)
		}
	}

	if got, _ := rec.Data["class_name"].(string); got != "Formacao Florestal" {
		t.Errorf("class_name = %q, want Formacao Florestal", got)
	}
	if got, _ := rec.Data["territory_name"].(string); got != "Brasil" {
		t.Errorf("territory_name = %q, want Brasil", got)
	}
}

func TestMapBiomasCollector_Collect_EmptyResponse(t *testing.T) {
	emptyResp := map[string]any{
		"data": []any{},
	}
	srv := newMapBiomasServer(t, emptyResp, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewMapBiomasCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestMapBiomasCollector_Collect_HTTPError(t *testing.T) {
	srv := newMapBiomasServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := ambiental.NewMapBiomasCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}
