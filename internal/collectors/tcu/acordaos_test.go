package tcu_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/tcu"
)

func TestAcordaosCollector_SourceAndSchedule(t *testing.T) {
	c := tcu.NewAcordaosCollector("")
	if c.Source() != "tcu_acordaos" {
		t.Errorf("Source() = %q, want tcu_acordaos", c.Source())
	}
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestAcordaosCollector_Collect_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"tipo":"ACÓRDÃO","numero":"123","anoAcordao":"2026"}]`))
	}))
	defer srv.Close()

	c := tcu.NewAcordaosCollectorWithURL(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Source != "tcu_acordaos" {
		t.Errorf("Source = %q, want tcu_acordaos", records[0].Source)
	}
}

func TestAcordaosCollector_Collect_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := tcu.NewAcordaosCollectorWithURL(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 upstream, got nil")
	}
}
