package b3_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/b3"
)

var fakeIndicesResponse = map[string]any{
	"results": []map[string]any{
		{
			"cod":    "PETR4",
			"asset":  "PETROBRAS PN",
			"part":   "8.234",
			"theorQtd": float64(1234567),
		},
		{
			"cod":    "VALE3",
			"asset":  "VALE ON NM",
			"part":   "12.567",
			"theorQtd": float64(9876543),
		},
	},
}

func newIndicesServer(t *testing.T, resp any, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestIndicesCollector_Source(t *testing.T) {
	srv := newIndicesServer(t, fakeIndicesResponse, http.StatusOK)
	defer srv.Close()

	c := b3.NewIndicesCollector(srv.URL)
	if got := c.Source(); got != "b3_ibovespa" {
		t.Errorf("Source() = %q, want %q", got, "b3_ibovespa")
	}
}

func TestIndicesCollector_Schedule(t *testing.T) {
	srv := newIndicesServer(t, fakeIndicesResponse, http.StatusOK)
	defer srv.Close()

	c := b3.NewIndicesCollector(srv.URL)
	if got := c.Schedule(); got != "0 18 * * 1-5" {
		t.Errorf("Schedule() = %q, want %q", got, "0 18 * * 1-5")
	}
}

func TestIndicesCollector_Collect(t *testing.T) {
	srv := newIndicesServer(t, fakeIndicesResponse, http.StatusOK)
	defer srv.Close()

	c := b3.NewIndicesCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record (aggregated), got %d", len(records))
	}

	r := records[0]
	if r.Source != "b3_ibovespa" {
		t.Errorf("Source = %q, want b3_ibovespa", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["indice"]; !ok {
		t.Error("Data must contain 'indice' field")
	}
	if _, ok := r.Data["composicao"]; !ok {
		t.Error("Data must contain 'composicao' field")
	}
	if _, ok := r.Data["total_ativos"]; !ok {
		t.Error("Data must contain 'total_ativos' field")
	}
}

func TestIndicesCollector_EmptyResponse(t *testing.T) {
	empty := map[string]any{"results": []map[string]any{}}
	srv := newIndicesServer(t, empty, http.StatusOK)
	defer srv.Close()

	c := b3.NewIndicesCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestIndicesCollector_HTTPError(t *testing.T) {
	srv := newIndicesServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := b3.NewIndicesCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}
