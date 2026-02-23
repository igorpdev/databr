package bcb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/bcb"
)

var fakeSelicResponse = []map[string]any{
	{"data": "20/02/2026", "valor": "0.055131"},
	{"data": "19/02/2026", "valor": "0.055131"},
	{"data": "18/02/2026", "valor": "0.055131"},
}

func newSGSServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestSelicCollector_Source(t *testing.T) {
	srv := newSGSServer(t, fakeSelicResponse)
	defer srv.Close()
	c := bcb.NewSelicCollector(srv.URL)
	if c.Source() != "bcb_selic" {
		t.Errorf("Source() = %q, want %q", c.Source(), "bcb_selic")
	}
}

func TestSelicCollector_Schedule(t *testing.T) {
	srv := newSGSServer(t, fakeSelicResponse)
	defer srv.Close()
	c := bcb.NewSelicCollector(srv.URL)
	if c.Schedule() != "0 13 * * 1-5" {
		t.Errorf("Schedule() = %q, want 0 13 * * 1-5", c.Schedule())
	}
}

func TestSelicCollector_Collect(t *testing.T) {
	srv := newSGSServer(t, fakeSelicResponse)
	defer srv.Close()

	c := bcb.NewSelicCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Collect() returned 0 records")
	}

	r := records[0]
	if r.Source != "bcb_selic" {
		t.Errorf("Source = %q, want bcb_selic", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["valor"]; !ok {
		t.Error("Data must contain 'valor' field")
	}
	if _, ok := r.Data["data"]; !ok {
		t.Error("Data must contain 'data' field")
	}
}

func TestSelicCollector_EmptyResponse(t *testing.T) {
	srv := newSGSServer(t, []map[string]any{})
	defer srv.Close()

	c := bcb.NewSelicCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}
