package bcb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/bcb"
)

var fakePTAXResponse = map[string]any{
	"value": []map[string]any{
		{
			"cotacaoCompra":    5.75,
			"cotacaoVenda":     5.76,
			"dataHoraCotacao":  "2026-02-20 13:00:00.000",
		},
	},
}

var fakePTAXEmptyResponse = map[string]any{
	"value": []map[string]any{}, // weekends return empty
}

func newOLINDAServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestPTAXCollector_Source(t *testing.T) {
	srv := newOLINDAServer(t, fakePTAXResponse)
	defer srv.Close()
	c := bcb.NewPTAXCollector(srv.URL)
	if c.Source() != "bcb_ptax" {
		t.Errorf("Source() = %q, want bcb_ptax", c.Source())
	}
}

func TestPTAXCollector_Schedule(t *testing.T) {
	srv := newOLINDAServer(t, fakePTAXResponse)
	defer srv.Close()
	c := bcb.NewPTAXCollector(srv.URL)
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestPTAXCollector_FetchUSD(t *testing.T) {
	srv := newOLINDAServer(t, fakePTAXResponse)
	defer srv.Close()

	c := bcb.NewPTAXCollector(srv.URL)
	records, err := c.FetchByCurrency(context.Background(), "USD", "2026-02-20")
	if err != nil {
		t.Fatalf("FetchByCurrency() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	r := records[0]
	if r.Source != "bcb_ptax" {
		t.Errorf("Source = %q, want bcb_ptax", r.Source)
	}
	for _, field := range []string{"cotacao_compra", "cotacao_venda", "moeda", "data"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestPTAXCollector_WeekendReturnsEmpty(t *testing.T) {
	srv := newOLINDAServer(t, fakePTAXEmptyResponse)
	defer srv.Close()

	c := bcb.NewPTAXCollector(srv.URL)
	records, err := c.FetchByCurrency(context.Background(), "USD", "2026-02-22") // Sunday
	if err != nil {
		t.Fatalf("FetchByCurrency() error: %v", err)
	}
	// Empty is valid for weekends — returns no records, no error
	if len(records) != 0 {
		t.Errorf("expected 0 records on weekend, got %d", len(records))
	}
}
