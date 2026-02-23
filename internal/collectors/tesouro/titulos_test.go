package tesouro_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/tesouro"
)

const fakeTitulosCSV = `Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha
Tesouro SELIC 2027;01/03/2027;19/02/2026;10,25;10,35;14891,39;14850,00;14850,00
Tesouro IPCA+ 2035;15/05/2035;19/02/2026;6,45;6,55;3456,78;3400,00;3400,00`

func newTitulosServer(t *testing.T, csvContent string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(csvContent))
	}))
}

func TestTesouroDiretoCollector_Source(t *testing.T) {
	srv := newTitulosServer(t, fakeTitulosCSV)
	defer srv.Close()
	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	if c.Source() != "tesouro_titulos" {
		t.Errorf("Source() = %q, want tesouro_titulos", c.Source())
	}
}

func TestTesouroDiretoCollector_Schedule(t *testing.T) {
	srv := newTitulosServer(t, fakeTitulosCSV)
	defer srv.Close()
	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestTesouroDiretoCollector_Collect(t *testing.T) {
	srv := newTitulosServer(t, fakeTitulosCSV)
	defer srv.Close()

	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "tesouro_titulos" {
		t.Errorf("Source = %q, want tesouro_titulos", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	for _, field := range []string{"nome", "vencimento", "data_base", "taxa_compra", "taxa_venda", "pu_compra", "pu_venda", "pu_base"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestTesouroDiretoCollector_RecordKey(t *testing.T) {
	srv := newTitulosServer(t, fakeTitulosCSV)
	defer srv.Close()

	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// RecordKey: "{tipo with spaces→underscores}_{dataVenc with /→-}"
	expected := "Tesouro_SELIC_2027_01-03-2027"
	if records[0].RecordKey != expected {
		t.Errorf("RecordKey = %q, want %q", records[0].RecordKey, expected)
	}
}

func TestTesouroDiretoCollector_EmptyResponse(t *testing.T) {
	// Header-only CSV should return an error (no records)
	srv := newTitulosServer(t, "Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha\n")
	defer srv.Close()

	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty CSV, got nil")
	}
}

func TestTesouroDiretoCollector_LatestDateOnly(t *testing.T) {
	// CSV with two dates: only the latest (20/02/2026) should be returned
	csvContent := `Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha
Tesouro SELIC 2027;01/03/2027;18/02/2026;10,10;10,20;14800,00;14780,00;14780,00
Tesouro SELIC 2027;01/03/2027;20/02/2026;10,25;10,35;14891,39;14850,00;14850,00
Tesouro IPCA+ 2035;15/05/2035;20/02/2026;6,45;6,55;3456,78;3400,00;3400,00`

	srv := newTitulosServer(t, csvContent)
	defer srv.Close()

	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records for latest date, got %d", len(records))
	}
	for _, r := range records {
		if r.Data["data_base"] != "20/02/2026" {
			t.Errorf("expected data_base=20/02/2026, got %v", r.Data["data_base"])
		}
	}
}
