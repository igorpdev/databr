package transparencia_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/transparencia"
)

var fakeCEISResponse = map[string]any{
	"data": []map[string]any{
		{
			"cnpj":              "12345678000195",
			"nomeEmpresa":       "EMPRESA XPTO LTDA",
			"dataPublicacao":    "2025-01-10",
			"tipoSancao":        "Inidoneidade",
			"orgaoSancionador":  "CGU",
		},
	},
}

func newCGUServer(t *testing.T, endpoint string, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate API key header
		if r.Header.Get("chave-api-dados") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestCGUCollector_Source(t *testing.T) {
	srv := newCGUServer(t, "/ceis", fakeCEISResponse)
	defer srv.Close()
	c := transparencia.NewCGUCollector(srv.URL, "test-api-key")
	if c.Source() != "cgu_compliance" {
		t.Errorf("Source() = %q, want cgu_compliance", c.Source())
	}
}

func TestCGUCollector_Schedule(t *testing.T) {
	srv := newCGUServer(t, "/ceis", fakeCEISResponse)
	defer srv.Close()
	c := transparencia.NewCGUCollector(srv.URL, "test-api-key")
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestCGUCollector_FetchCEIS(t *testing.T) {
	srv := newCGUServer(t, "/ceis", fakeCEISResponse)
	defer srv.Close()

	c := transparencia.NewCGUCollector(srv.URL, "test-api-key")
	records, err := c.FetchByCNPJ(context.Background(), "12345678000195")
	if err != nil {
		t.Fatalf("FetchByCNPJ() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	r := records[0]
	if r.Source != "cgu_compliance" {
		t.Errorf("Source = %q, want cgu_compliance", r.Source)
	}
	if _, ok := r.Data["ceis"]; !ok {
		t.Error("Data must contain 'ceis' key")
	}
	if _, ok := r.Data["cnpj"]; !ok {
		t.Error("Data must contain 'cnpj' key")
	}
}

func TestCGUCollector_MissingAPIKey(t *testing.T) {
	srv := newCGUServer(t, "/ceis", fakeCEISResponse)
	defer srv.Close()

	// Empty API key — should error or return empty (not crash)
	c := transparencia.NewCGUCollector(srv.URL, "")
	_, err := c.FetchByCNPJ(context.Background(), "12345678000195")
	if err == nil {
		t.Error("expected error when API key is missing")
	}
}
