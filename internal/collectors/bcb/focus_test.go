package bcb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/collectors/bcb"
)

// fakeFocusResponse mimics the OLINDA ExpectativasMercadoAnuais JSON structure.
var fakeFocusResponse = map[string]any{
	"value": []map[string]any{
		{
			"Indicador":          "IPCA",
			"IndicadorDetalhe":   "",
			"Data":               "2026-02-21",
			"DataReferencia":     "2026",
			"Media":              4.80,
			"Mediana":            4.75,
			"DesvioPadrao":       0.30,
			"Minimo":             4.10,
			"Maximo":             5.50,
			"numeroRespondentes": 120,
			"baseCalculo":        0,
		},
		{
			"Indicador":          "PIB Total",
			"IndicadorDetalhe":   "",
			"Data":               "2026-02-21",
			"DataReferencia":     "2026",
			"Media":              2.10,
			"Mediana":            2.00,
			"DesvioPadrao":       0.50,
			"Minimo":             0.80,
			"Maximo":             3.50,
			"numeroRespondentes": 110,
			"baseCalculo":        0,
		},
		{
			"Indicador":          "Câmbio",
			"IndicadorDetalhe":   "",
			"Data":               "2026-02-21",
			"DataReferencia":     "2026",
			"Media":              5.90,
			"Mediana":            5.85,
			"DesvioPadrao":       0.20,
			"Minimo":             5.40,
			"Maximo":             6.50,
			"numeroRespondentes": 100,
			"baseCalculo":        0,
		},
	},
}

var fakeFocusEmptyResponse = map[string]any{
	"value": []map[string]any{},
}

func newFocusServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestFocusCollector_Source(t *testing.T) {
	srv := newFocusServer(t, fakeFocusResponse)
	defer srv.Close()
	c := bcb.NewFocusCollector(srv.URL)
	if c.Source() != "bcb_focus" {
		t.Errorf("Source() = %q, want %q", c.Source(), "bcb_focus")
	}
}

func TestFocusCollector_Schedule(t *testing.T) {
	srv := newFocusServer(t, fakeFocusResponse)
	defer srv.Close()
	c := bcb.NewFocusCollector(srv.URL)
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestFocusCollector_Collect(t *testing.T) {
	srv := newFocusServer(t, fakeFocusResponse)
	defer srv.Close()

	c := bcb.NewFocusCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Collect() returned 0 records, want >0")
	}

	r := records[0]
	if r.Source != "bcb_focus" {
		t.Errorf("Source = %q, want bcb_focus", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	// RecordKey should follow pattern "INDICADOR_YEAR"
	if !strings.Contains(r.RecordKey, "_") {
		t.Errorf("RecordKey %q does not follow INDICADOR_YEAR pattern", r.RecordKey)
	}

	expectedFields := []string{"indicador", "data_referencia", "data", "mediana", "media", "minimo", "maximo", "numero_respondentes"}
	for _, field := range expectedFields {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestFocusCollector_EmptyValue(t *testing.T) {
	srv := newFocusServer(t, fakeFocusEmptyResponse)
	defer srv.Close()

	c := bcb.NewFocusCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for empty response, got %d", len(records))
	}
}

func TestFocusCollector_RecordKeyFormat(t *testing.T) {
	srv := newFocusServer(t, fakeFocusResponse)
	defer srv.Close()

	c := bcb.NewFocusCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	seen := make(map[string]bool)
	for _, r := range records {
		if seen[r.RecordKey] {
			t.Errorf("duplicate RecordKey: %q", r.RecordKey)
		}
		seen[r.RecordKey] = true
	}
}
