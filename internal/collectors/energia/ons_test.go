package energia_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/energia"
)

var fakeONSResponse = map[string]any{
	"success": true,
	"result": map[string]any{
		"records": []map[string]any{
			{
				"nom_subsistema":    "SE/CO",
				"din_instante":      "2026-02-22T10:00:00",
				"val_geracao":       float64(45230.5),
				"nom_tipousina":     "Hidroeletrica",
				"nom_usina":         "Itaipu",
			},
			{
				"nom_subsistema":    "S",
				"din_instante":      "2026-02-22T10:00:00",
				"val_geracao":       float64(12500.3),
				"nom_tipousina":     "Eolica",
				"nom_usina":         "Parque Eolico Sul",
			},
		},
	},
}

func newONSServer(t *testing.T, resp any, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestONSCollector_Source(t *testing.T) {
	srv := newONSServer(t, fakeONSResponse, http.StatusOK)
	defer srv.Close()

	c := energia.NewONSCollector(srv.URL)
	if got := c.Source(); got != "ons_geracao" {
		t.Errorf("Source() = %q, want %q", got, "ons_geracao")
	}
}

func TestONSCollector_Schedule(t *testing.T) {
	srv := newONSServer(t, fakeONSResponse, http.StatusOK)
	defer srv.Close()

	c := energia.NewONSCollector(srv.URL)
	if got := c.Schedule(); got != "0 9 * * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 9 * * *")
	}
}

func TestONSCollector_Collect(t *testing.T) {
	srv := newONSServer(t, fakeONSResponse, http.StatusOK)
	defer srv.Close()

	c := energia.NewONSCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "ons_geracao" {
		t.Errorf("Source = %q, want ons_geracao", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["nom_subsistema"]; !ok {
		t.Error("Data must contain 'nom_subsistema' field")
	}
	if _, ok := r.Data["val_geracao"]; !ok {
		t.Error("Data must contain 'val_geracao' field")
	}
}

func TestONSCollector_EmptyResponse(t *testing.T) {
	empty := map[string]any{
		"success": true,
		"result": map[string]any{
			"records": []map[string]any{},
		},
	}
	srv := newONSServer(t, empty, http.StatusOK)
	defer srv.Close()

	c := energia.NewONSCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestONSCollector_SuccessFalse(t *testing.T) {
	failed := map[string]any{
		"success": false,
		"result": map[string]any{
			"records": []map[string]any{},
		},
	}
	srv := newONSServer(t, failed, http.StatusOK)
	defer srv.Close()

	c := energia.NewONSCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for success=false, got nil")
	}
}

func TestONSCollector_HTTPError(t *testing.T) {
	srv := newONSServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := energia.NewONSCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestONSCollector_RecordKeyUnique(t *testing.T) {
	srv := newONSServer(t, fakeONSResponse, http.StatusOK)
	defer srv.Close()

	c := energia.NewONSCollector(srv.URL)
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
