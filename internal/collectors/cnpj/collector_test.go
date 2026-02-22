package cnpj_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/cnpj"
)

// fakeCNPJResponse simulates a minhareceita.org response
var fakeCNPJResponse = map[string]any{
	"cnpj":                "12345678000195",
	"razao_social":        "EMPRESA XPTO LTDA",
	"nome_fantasia":       "XPTO",
	"situacao_cadastral":  "ATIVA",
	"porte":               "MICRO EMPRESA",
	"municipio":           "SAO PAULO",
	"uf":                  "SP",
	"cnae_fiscal":         6201500,
	"capital_social":      100000.0,
	"data_inicio_atividade": "2015-03-10",
}

func newMinhaReceitaServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeCNPJResponse)
	}))
}

func TestCNPJCollector_Source(t *testing.T) {
	srv := newMinhaReceitaServer(t)
	defer srv.Close()

	c := cnpj.NewCollector(srv.URL)
	if c.Source() != "cnpj" {
		t.Errorf("Source() = %q, want %q", c.Source(), "cnpj")
	}
}

func TestCNPJCollector_Schedule(t *testing.T) {
	srv := newMinhaReceitaServer(t)
	defer srv.Close()

	c := cnpj.NewCollector(srv.URL)
	// CNPJ is on-demand, no background sync
	if c.Schedule() != "" {
		t.Errorf("Schedule() = %q, want empty (on-demand only)", c.Schedule())
	}
}

func TestCNPJCollector_FetchByCNPJ(t *testing.T) {
	srv := newMinhaReceitaServer(t)
	defer srv.Close()

	c := cnpj.NewCollector(srv.URL)

	records, err := c.FetchByCNPJ(context.Background(), "12345678000195")
	if err != nil {
		t.Fatalf("FetchByCNPJ() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("FetchByCNPJ() returned %d records, want 1", len(records))
	}

	r := records[0]
	if r.Source != "cnpj" {
		t.Errorf("Source = %q, want %q", r.Source, "cnpj")
	}
	if r.RecordKey != "12345678000195" {
		t.Errorf("RecordKey = %q, want %q", r.RecordKey, "12345678000195")
	}

	// Key fields must be present in normalized data
	for _, field := range []string{"cnpj", "razao_social", "situacao_cadastral", "uf", "municipio"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestCNPJCollector_FetchByCNPJ_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"CNPJ não encontrado"}`))
	}))
	defer srv.Close()

	c := cnpj.NewCollector(srv.URL)
	_, err := c.FetchByCNPJ(context.Background(), "00000000000000")
	if err == nil {
		t.Error("expected error for 404 response, got nil")
	}
}

func TestCNPJCollector_NormalizeCNPJ(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"12.345.678/0001-95", "12345678000195"},
		{"12345678000195", "12345678000195"},
		{"12 345 678/0001-95", "12345678000195"},
	}
	for _, tt := range tests {
		got := cnpj.NormalizeCNPJ(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeCNPJ(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
