package bcb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/bcb"
)

var fakeTaxasResponse = map[string]any{
	"value": []map[string]any{
		{
			"Mes":                  "2025-01",
			"anoMes":               "202501",
			"Modalidade":           "Crédito pessoal não consignado",
			"InstituicaoFinanceira": "Banco Itaú",
			"cnpj8":                "60872504",
			"TaxaJurosAoMes":       7.23,
			"TaxaJurosAoAno":       130.45,
		},
		{
			"Mes":                  "2025-01",
			"anoMes":               "202501",
			"Modalidade":           "Cartão de crédito total",
			"InstituicaoFinanceira": "Banco Bradesco",
			"cnpj8":                "60746948",
			"TaxaJurosAoMes":       15.12,
			"TaxaJurosAoAno":       432.18,
		},
	},
}

func newTaxasServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestTaxasCreditoCollector_Source(t *testing.T) {
	srv := newTaxasServer(t, fakeTaxasResponse)
	defer srv.Close()
	c := bcb.NewTaxasCreditoCollector(srv.URL)
	if c.Source() != "bcb_taxas_credito" {
		t.Errorf("Source() = %q, want bcb_taxas_credito", c.Source())
	}
}

func TestTaxasCreditoCollector_Schedule(t *testing.T) {
	srv := newTaxasServer(t, fakeTaxasResponse)
	defer srv.Close()
	c := bcb.NewTaxasCreditoCollector(srv.URL)
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestTaxasCreditoCollector_Collect(t *testing.T) {
	srv := newTaxasServer(t, fakeTaxasResponse)
	defer srv.Close()

	c := bcb.NewTaxasCreditoCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "bcb_taxas_credito" {
		t.Errorf("Source = %q, want bcb_taxas_credito", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	for _, field := range []string{"mes", "ano_mes", "modalidade", "instituicao", "cnpj8", "taxa_mensal", "taxa_anual"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestTaxasCreditoCollector_EmptyResponse(t *testing.T) {
	srv := newTaxasServer(t, map[string]any{"value": []map[string]any{}})
	defer srv.Close()

	c := bcb.NewTaxasCreditoCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestTaxasCreditoCollector_RecordKey(t *testing.T) {
	srv := newTaxasServer(t, fakeTaxasResponse)
	defer srv.Close()

	c := bcb.NewTaxasCreditoCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// RecordKey: "{cnpj8}_{anoMes}_{modalidade}"
	expected := "60872504_202501_Crédito pessoal não consignado"
	if records[0].RecordKey != expected {
		t.Errorf("RecordKey = %q, want %q", records[0].RecordKey, expected)
	}
}
