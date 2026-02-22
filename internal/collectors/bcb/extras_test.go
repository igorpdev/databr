package bcb_test

import (
	"context"
	"testing"

	"github.com/databr/api/internal/collectors/bcb"
)

var fakeCreditoResponse = []map[string]any{
	{"data": "01/12/2025", "valor": "4090464.00"},
}

func TestCreditoCollector_Source(t *testing.T) {
	srv := newSGSServer(t, fakeCreditoResponse)
	defer srv.Close()
	c := bcb.NewCreditoCollector(srv.URL)
	if c.Source() != "bcb_credito" {
		t.Errorf("Source() = %q, want bcb_credito", c.Source())
	}
}

func TestCreditoCollector_Collect(t *testing.T) {
	srv := newSGSServer(t, fakeCreditoResponse)
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
