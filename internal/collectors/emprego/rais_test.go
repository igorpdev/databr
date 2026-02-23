package emprego_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/collectors/emprego"
)

var fakeRAISResponse = []map[string]any{
	{
		"sector_code": "A01",
		"sector_name": "Agricultura",
		"employees":   150000,
		"avg_salary":  2500.50,
		"admissions":  30000,
		"dismissals":  25000,
		"uf":          "SP",
	},
	{
		"sector_code": "C10",
		"sector_name": "Industria de Alimentos",
		"employees":   80000,
		"avg_salary":  3200.00,
		"admissions":  15000,
		"dismissals":  12000,
		"uf":          "MG",
	},
	{
		"sector_code": "",
		"sector_name": "Setor Invalido",
		"employees":   0,
		"avg_salary":  0,
		"admissions":  0,
		"dismissals":  0,
		"uf":          "",
	},
}

func newRAISServer(t *testing.T, resp any, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestRAISCollector_Source(t *testing.T) {
	c := emprego.NewRAISCollector("http://localhost")
	if got := c.Source(); got != "rais_emprego" {
		t.Errorf("Source() = %q, want %q", got, "rais_emprego")
	}
}

func TestRAISCollector_Schedule(t *testing.T) {
	c := emprego.NewRAISCollector("http://localhost")
	if got := c.Schedule(); got != "0 3 1 * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 3 1 * *")
	}
}

func TestRAISCollector_Collect(t *testing.T) {
	srv := newRAISServer(t, fakeRAISResponse, http.StatusOK)
	defer srv.Close()

	c := emprego.NewRAISCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Expect 2 valid records (empty sector_code is skipped).
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestRAISCollector_Collect_RecordKey(t *testing.T) {
	srv := newRAISServer(t, fakeRAISResponse, http.StatusOK)
	defer srv.Close()

	c := emprego.NewRAISCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	year := time.Now().Year() - 1
	expectedKey0 := fmt.Sprintf("%d_A01", year)
	if records[0].RecordKey != expectedKey0 {
		t.Errorf("records[0].RecordKey = %q, want %q", records[0].RecordKey, expectedKey0)
	}
}

func TestRAISCollector_Collect_SourceField(t *testing.T) {
	srv := newRAISServer(t, fakeRAISResponse, http.StatusOK)
	defer srv.Close()

	c := emprego.NewRAISCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.Source != "rais_emprego" {
			t.Errorf("Source = %q, want %q", rec.Source, "rais_emprego")
		}
	}
}

func TestRAISCollector_Collect_DataFields(t *testing.T) {
	srv := newRAISServer(t, fakeRAISResponse, http.StatusOK)
	defer srv.Close()

	c := emprego.NewRAISCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one record")
	}

	rec := records[0]
	required := []string{
		"year", "sector_code", "sector_name", "employees",
		"avg_salary", "admissions", "dismissals", "uf",
	}
	for _, field := range required {
		if _, ok := rec.Data[field]; !ok {
			t.Errorf("Data must contain field %q", field)
		}
	}

	// Spot-check values.
	if got, _ := rec.Data["sector_code"].(string); got != "A01" {
		t.Errorf("sector_code = %q, want A01", got)
	}
	if got, _ := rec.Data["uf"].(string); got != "SP" {
		t.Errorf("uf = %q, want SP", got)
	}
}

func TestRAISCollector_Collect_SkipsEmptySectorCode(t *testing.T) {
	srv := newRAISServer(t, fakeRAISResponse, http.StatusOK)
	defer srv.Close()

	c := emprego.NewRAISCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.RecordKey == "" {
			t.Error("found record with empty RecordKey; should have been skipped")
		}
	}
}

func TestRAISCollector_Collect_HTTPError(t *testing.T) {
	srv := newRAISServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := emprego.NewRAISCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestRAISCollector_Collect_EmptyResponse(t *testing.T) {
	srv := newRAISServer(t, []map[string]any{}, http.StatusOK)
	defer srv.Close()

	c := emprego.NewRAISCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("empty response should not error, got: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}
