package tributario

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIBPTCollector_Source(t *testing.T) {
	c := NewIBPTCollector("")
	if got := c.Source(); got != "ibpt_tributos" {
		t.Errorf("Source() = %q, want %q", got, "ibpt_tributos")
	}
}

func TestIBPTCollector_Schedule(t *testing.T) {
	c := NewIBPTCollector("")
	if got := c.Schedule(); got != "" {
		t.Errorf("Schedule() = %q, want empty string", got)
	}
}

func TestIBPTCollector_FetchByNCM_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("codigo") != "22030000" {
			t.Errorf("expected codigo=22030000, got %s", r.URL.Query().Get("codigo"))
		}
		if r.URL.Query().Get("uf") != "SP" {
			t.Errorf("expected uf=SP, got %s", r.URL.Query().Get("uf"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"codigo":            "22030000",
			"ex":                "",
			"tipo":              0,
			"descricao":         "Cervejas de malte",
			"nacionalfederal":   "13.91",
			"importadosfederal": "17.77",
			"estadual":          "22.00",
			"municipal":         "0.00",
			"vigenciainicio":    "2026-01-20",
			"vigenciafim":       "2026-02-28",
			"versao":            "26.1.C",
			"fonte":             "IBPT/empresometro.com.br",
			"uf":                "SP",
		})
	}))
	defer srv.Close()

	c := NewIBPTCollector(srv.URL)
	records, err := c.FetchByNCM(context.Background(), "22030000", "SP")
	if err != nil {
		t.Fatalf("FetchByNCM() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Source != "ibpt_tributos" {
		t.Errorf("Source = %q, want %q", rec.Source, "ibpt_tributos")
	}
	if rec.RecordKey != "22030000_sp" {
		t.Errorf("RecordKey = %q, want %q", rec.RecordKey, "22030000_sp")
	}

	aliquotas, ok := rec.Data["aliquotas"].(map[string]any)
	if !ok {
		t.Fatal("expected aliquotas map in Data")
	}
	if fedNac, ok := aliquotas["federal_nacional"].(float64); !ok || fedNac != 13.91 {
		t.Errorf("federal_nacional = %v, want 13.91", aliquotas["federal_nacional"])
	}
	if totalNac, ok := aliquotas["total_nacional"].(float64); !ok || totalNac != 35.91 {
		t.Errorf("total_nacional = %v, want 35.91", aliquotas["total_nacional"])
	}
	if rec.Data["tipo"] != "ncm" {
		t.Errorf("tipo = %v, want 'ncm'", rec.Data["tipo"])
	}
}

func TestIBPTCollector_FetchByNCM_ServiceType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"codigo":            "0107",
			"tipo":              2,
			"descricao":         "Suporte tecnico em informatica",
			"nacionalfederal":   "13.45",
			"importadosfederal": "15.45",
			"estadual":          "0.00",
			"municipal":         "2.70",
			"vigenciainicio":    "2026-01-20",
			"vigenciafim":       "2026-02-28",
			"versao":            "26.1.C",
			"fonte":             "IBPT",
			"uf":                "SP",
		})
	}))
	defer srv.Close()

	c := NewIBPTCollector(srv.URL)
	records, err := c.FetchByNCM(context.Background(), "0107", "SP")
	if err != nil {
		t.Fatalf("FetchByNCM() error: %v", err)
	}
	if records[0].Data["tipo"] != "servico" {
		t.Errorf("tipo = %v, want 'servico'", records[0].Data["tipo"])
	}
}

func TestIBPTCollector_FetchByNCM_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Nenhum registro encontrado",
				"code":    404,
			},
		})
	}))
	defer srv.Close()

	c := NewIBPTCollector(srv.URL)
	_, err := c.FetchByNCM(context.Background(), "99999999", "SP")
	if err == nil {
		t.Fatal("expected error for not found NCM")
	}
}

func TestIBPTCollector_FetchByNCM_ValidationErrors(t *testing.T) {
	c := NewIBPTCollector("http://unused")

	tests := []struct {
		name    string
		codigo  string
		uf      string
	}{
		{"empty codigo", "", "SP"},
		{"empty uf", "22030000", ""},
		{"invalid uf length", "22030000", "SPA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.FetchByNCM(context.Background(), tt.codigo, tt.uf)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}
