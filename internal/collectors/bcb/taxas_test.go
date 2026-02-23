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
			"Segmento":        "Pessoa Física",
			"Modalidade":      "Crédito pessoal não consignado",
			"Posicao":         "A vista",
			"DataReferencia":  "2025-01",
			"TaxaJurosMensal": 7.23,
			"TaxaJurosAnual":  130.45,
		},
		{
			"Segmento":        "Pessoa Física",
			"Modalidade":      "Cartão de crédito total",
			"Posicao":         "A vista",
			"DataReferencia":  "2025-01",
			"TaxaJurosMensal": 15.12,
			"TaxaJurosAnual":  432.18,
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
	for _, field := range []string{"segmento", "modalidade", "posicao", "data_referencia", "taxa_mensal", "taxa_anual"} {
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

	// RecordKey should contain both modalidade and data_referencia
	expected := "Crédito pessoal não consignado_2025-01"
	if records[0].RecordKey != expected {
		t.Errorf("RecordKey = %q, want %q", records[0].RecordKey, expected)
	}
}
