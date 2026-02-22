package bcb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/bcb"
)

var fakeOLINDAResponse = map[string]any{
	"value": []map[string]any{
		{"Data": "2026-01", "Valor": "5800.5"},
	},
}

func newOLINDAGenServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestCreditoCollector_Source(t *testing.T) {
	srv := newOLINDAGenServer(t, fakeOLINDAResponse)
	defer srv.Close()
	c := bcb.NewCreditoCollector(srv.URL)
	if c.Source() != "bcb_credito" {
		t.Errorf("Source() = %q, want bcb_credito", c.Source())
	}
}

func TestCreditoCollector_Collect(t *testing.T) {
	srv := newOLINDAGenServer(t, fakeOLINDAResponse)
	defer srv.Close()

	c := bcb.NewCreditoCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].Source != "bcb_credito" {
		t.Errorf("Source = %q, want bcb_credito", records[0].Source)
	}
}

var fakeSGSResponse = []map[string]any{
	{"data": "01/02/2026", "valor": "4200000.00"},
}

func TestReservasCollector_Source(t *testing.T) {
	srv := newSGSServer(t, fakeSGSResponse)
	defer srv.Close()
	c := bcb.NewReservasCollector(srv.URL)
	if c.Source() != "bcb_reservas" {
		t.Errorf("Source() = %q, want bcb_reservas", c.Source())
	}
}

func TestReservasCollector_Collect(t *testing.T) {
	srv := newSGSServer(t, fakeSGSResponse)
	defer srv.Close()

	c := bcb.NewReservasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
}
