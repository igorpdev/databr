package educacao_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/educacao"
)

var fakeINEPResponse = map[string]any{
	"success": true,
	"result": map[string]any{
		"records": []map[string]any{
			{
				"ano":              "2025",
				"indicador":        "taxa_matricula",
				"regiao":           "Sudeste",
				"valor":            float64(95.3),
				"nivel_ensino":     "fundamental",
			},
			{
				"ano":              "2025",
				"indicador":        "taxa_abandono",
				"regiao":           "Norte",
				"valor":            float64(3.2),
				"nivel_ensino":     "medio",
			},
		},
	},
}

func newINEPServer(t *testing.T, resp any, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestINEPCollector_Source(t *testing.T) {
	srv := newINEPServer(t, fakeINEPResponse, http.StatusOK)
	defer srv.Close()

	c := educacao.NewINEPCollector(srv.URL)
	if got := c.Source(); got != "inep_censo_escolar" {
		t.Errorf("Source() = %q, want %q", got, "inep_censo_escolar")
	}
}

func TestINEPCollector_Schedule(t *testing.T) {
	srv := newINEPServer(t, fakeINEPResponse, http.StatusOK)
	defer srv.Close()

	c := educacao.NewINEPCollector(srv.URL)
	if got := c.Schedule(); got != "0 8 1 * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 8 1 * *")
	}
}

func TestINEPCollector_Collect(t *testing.T) {
	srv := newINEPServer(t, fakeINEPResponse, http.StatusOK)
	defer srv.Close()

	c := educacao.NewINEPCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "inep_censo_escolar" {
		t.Errorf("Source = %q, want inep_censo_escolar", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["ano"]; !ok {
		t.Error("Data must contain 'ano' field")
	}
	if _, ok := r.Data["indicador"]; !ok {
		t.Error("Data must contain 'indicador' field")
	}
	if _, ok := r.Data["valor"]; !ok {
		t.Error("Data must contain 'valor' field")
	}
}

func TestINEPCollector_EmptyResponse(t *testing.T) {
	empty := map[string]any{
		"success": true,
		"result": map[string]any{
			"records": []map[string]any{},
		},
	}
	srv := newINEPServer(t, empty, http.StatusOK)
	defer srv.Close()

	c := educacao.NewINEPCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestINEPCollector_SuccessFalse(t *testing.T) {
	failed := map[string]any{
		"success": false,
		"result": map[string]any{
			"records": []map[string]any{},
		},
	}
	srv := newINEPServer(t, failed, http.StatusOK)
	defer srv.Close()

	c := educacao.NewINEPCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for success=false, got nil")
	}
}

func TestINEPCollector_HTTPError(t *testing.T) {
	srv := newINEPServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := educacao.NewINEPCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestINEPCollector_RecordKeyUnique(t *testing.T) {
	srv := newINEPServer(t, fakeINEPResponse, http.StatusOK)
	defer srv.Close()

	c := educacao.NewINEPCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	seen := make(map[string]bool)
	for _, r := range records {
		if seen[r.RecordKey] {
			t.Errorf("duplicate RecordKey: %q", r.RecordKey)
		}
		seen[r.RecordKey] = true
	}
}
