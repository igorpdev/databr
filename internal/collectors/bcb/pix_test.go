package bcb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/bcb"
)

func newPIXServer(t *testing.T) *httptest.Server {
	t.Helper()
	resp := map[string]any{
		"value": []map[string]any{
			{"AnoMes": "202501", "QtdTransacoes": 5000000000, "ValorTransacoes": 1200000000000.0},
			{"AnoMes": "202412", "QtdTransacoes": 4800000000, "ValorTransacoes": 1100000000000.0},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestPIXCollector_Source(t *testing.T) {
	srv := newPIXServer(t)
	defer srv.Close()
	c := bcb.NewPIXCollector(srv.URL)
	if c.Source() != "bcb_pix" {
		t.Errorf("Source() = %q, want bcb_pix", c.Source())
	}
}

func TestPIXCollector_Schedule(t *testing.T) {
	srv := newPIXServer(t)
	defer srv.Close()
	c := bcb.NewPIXCollector(srv.URL)
	if c.Schedule() != "@monthly" {
		t.Errorf("Schedule() = %q, want @monthly", c.Schedule())
	}
}

func TestPIXCollector_Collect(t *testing.T) {
	srv := newPIXServer(t)
	defer srv.Close()
	c := bcb.NewPIXCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected records")
	}
	r := records[0]
	if r.Source != "bcb_pix" {
		t.Errorf("Source = %q, want bcb_pix", r.Source)
	}
	if _, ok := r.Data["ano_mes"]; !ok {
		t.Error("missing field ano_mes")
	}
}
