package transparencia_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/transparencia"
)

// fakePNCPResponse mirrors the new /api/consulta/v1 paginated response format.
var fakePNCPResponse = map[string]any{
	"totalRegistros": 1,
	"totalPaginas":   1,
	"numeroPagina":   1,
	"data": []map[string]any{
		{
			"numeroCompra":      "PE1/2026",
			"anoCompra":         2026.0,
			"sequencialCompra":  1.0,
			"objetoCompra":      "Aquisição de material de escritório",
			"dataAtualizacao":   "2026-02-01T00:00:00",
			"orgaoEntidade": map[string]any{
				"cnpj":       "00000000000001",
				"razaoSocial": "MINISTÉRIO DA FAZENDA",
			},
		},
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
	if c.Schedule() != "0 22 * * 1-5" {
		t.Errorf("Schedule() = %q, want 0 22 * * 1-5", c.Schedule())
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
	for _, field := range []string{"numero_compra", "objeto", "orgao"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}
