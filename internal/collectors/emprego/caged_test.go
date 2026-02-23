package emprego_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/collectors/emprego"
)

var fakeCAGEDResponse = []map[string]any{
	{
		"municipio":    "SAO PAULO",
		"uf":           "SP",
		"sector_code":  "G47",
		"sector_name":  "Comercio Varejista",
		"admissions":   5000,
		"dismissals":   3000,
		"net_balance":  2000,
		"salario_medio": 2100.50,
	},
	{
		"municipio":    "RIO DE JANEIRO",
		"uf":           "RJ",
		"sector_code":  "I56",
		"sector_name":  "Alimentacao",
		"admissions":   3500,
		"dismissals":   4000,
		"net_balance":  -500,
		"salario_medio": 1800.00,
	},
}

func newCAGEDServer(t *testing.T, resp any, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestCAGEDCollector_Source(t *testing.T) {
	c := emprego.NewCAGEDCollector("http://localhost")
	if got := c.Source(); got != "caged_emprego" {
		t.Errorf("Source() = %q, want %q", got, "caged_emprego")
	}
}

func TestCAGEDCollector_Schedule(t *testing.T) {
	c := emprego.NewCAGEDCollector("http://localhost")
	if got := c.Schedule(); got != "0 12 1 * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 12 1 * *")
	}
}

func TestCAGEDCollector_Collect(t *testing.T) {
	srv := newCAGEDServer(t, fakeCAGEDResponse, http.StatusOK)
	defer srv.Close()

	c := emprego.NewCAGEDCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestCAGEDCollector_Collect_RecordKey(t *testing.T) {
	srv := newCAGEDServer(t, fakeCAGEDResponse, http.StatusOK)
	defer srv.Close()

	c := emprego.NewCAGEDCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	prevMonth := time.Now().AddDate(0, -1, 0)
	monthStr := prevMonth.Format("200601")
	expected0 := "caged_" + monthStr + "_0"
	expected1 := "caged_" + monthStr + "_1"

	if records[0].RecordKey != expected0 {
		t.Errorf("records[0].RecordKey = %q, want %q", records[0].RecordKey, expected0)
	}
	if records[1].RecordKey != expected1 {
		t.Errorf("records[1].RecordKey = %q, want %q", records[1].RecordKey, expected1)
	}
}

func TestCAGEDCollector_Collect_SourceField(t *testing.T) {
	srv := newCAGEDServer(t, fakeCAGEDResponse, http.StatusOK)
	defer srv.Close()

	c := emprego.NewCAGEDCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.Source != "caged_emprego" {
			t.Errorf("Source = %q, want %q", rec.Source, "caged_emprego")
		}
	}
}

func TestCAGEDCollector_Collect_DataFields(t *testing.T) {
	srv := newCAGEDServer(t, fakeCAGEDResponse, http.StatusOK)
	defer srv.Close()

	c := emprego.NewCAGEDCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one record")
	}

	rec := records[0]
	required := []string{
		"month", "municipio", "uf", "sector_code", "sector_name",
		"admissions", "dismissals", "net_balance", "salario_medio",
	}
	for _, field := range required {
		if _, ok := rec.Data[field]; !ok {
			t.Errorf("Data must contain field %q", field)
		}
	}

	// Spot-check values.
	if got, _ := rec.Data["municipio"].(string); got != "SAO PAULO" {
		t.Errorf("municipio = %q, want SAO PAULO", got)
	}
	if got, _ := rec.Data["uf"].(string); got != "SP" {
		t.Errorf("uf = %q, want SP", got)
	}
}

func TestCAGEDCollector_Collect_HTTPError(t *testing.T) {
	srv := newCAGEDServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := emprego.NewCAGEDCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestCAGEDCollector_Collect_EmptyResponse(t *testing.T) {
	srv := newCAGEDServer(t, []map[string]any{}, http.StatusOK)
	defer srv.Close()

	c := emprego.NewCAGEDCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("empty response should not error, got: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}
