package transparencia_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/transparencia"
)

var fakePNCPResponse = []map[string]any{
	{
		"cnpj":                 "00000000000001",
		"razaoSocial":          "MINISTÉRIO DA FAZENDA",
		"codigoUnidade":        "170001",
		"numeroControlePNCP":   "2026000001",
		"dataPublicacaoGlobal": "2026-02-01",
		"objeto":               "Aquisição de material de escritório",
		"valorTotalEstimado":   50000.0,
	},
}

func newPNCPServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestPNCPCollector_Source(t *testing.T) {
	srv := newPNCPServer(t, fakePNCPResponse)
	defer srv.Close()
	c := transparencia.NewPNCPCollector(srv.URL)
	if c.Source() != "pncp_licitacoes" {
		t.Errorf("Source() = %q, want pncp_licitacoes", c.Source())
	}
}

func TestPNCPCollector_Schedule(t *testing.T) {
	srv := newPNCPServer(t, fakePNCPResponse)
	defer srv.Close()
	c := transparencia.NewPNCPCollector(srv.URL)
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestPNCPCollector_Collect(t *testing.T) {
	srv := newPNCPServer(t, fakePNCPResponse)
	defer srv.Close()

	c := transparencia.NewPNCPCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	r := records[0]
	if r.Source != "pncp_licitacoes" {
		t.Errorf("Source = %q, want pncp_licitacoes", r.Source)
	}
	for _, field := range []string{"numero_controle", "objeto", "valor_estimado"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}
