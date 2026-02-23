package tesouro_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/collectors/tesouro"
)

var fakeTitulosResponse = map[string]any{
	"response": map[string]any{
		"TrsrBdTradgList": []map[string]any{
			{
				"TrsrBd": map[string]any{
					"nm":              "Tesouro SELIC 2027",
					"mtrtyDt":         "2027-03-01T00:00:00",
					"untrRedVal":      14891.39,
					"anulInvstmtRate": 0.0,
					"anulRedRate":     10.89,
					"minInvstmtAmt":   149.77,
				},
			},
			{
				"TrsrBd": map[string]any{
					"nm":              "Tesouro IPCA+ 2035",
					"mtrtyDt":         "2035-05-15T00:00:00",
					"untrRedVal":      3456.78,
					"anulInvstmtRate": 6.45,
					"anulRedRate":     6.42,
					"minInvstmtAmt":   34.57,
				},
			},
		},
	},
}

func newTitulosServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestTesouroDiretoCollector_Source(t *testing.T) {
	srv := newTitulosServer(t, fakeTitulosResponse)
	defer srv.Close()
	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	if c.Source() != "tesouro_titulos" {
		t.Errorf("Source() = %q, want tesouro_titulos", c.Source())
	}
}

func TestTesouroDiretoCollector_Schedule(t *testing.T) {
	srv := newTitulosServer(t, fakeTitulosResponse)
	defer srv.Close()
	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestTesouroDiretoCollector_Collect(t *testing.T) {
	srv := newTitulosServer(t, fakeTitulosResponse)
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
	// RecordKey should use underscores instead of spaces
	if strings.Contains(r.RecordKey, " ") {
		t.Errorf("RecordKey should not contain spaces, got %q", r.RecordKey)
	}
	for _, field := range []string{"nome", "vencimento", "taxa_anual_compra", "taxa_anual_resgate", "preco_minimo", "preco_unitario_resgate"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestTesouroDiretoCollector_RecordKey(t *testing.T) {
	srv := newTitulosServer(t, fakeTitulosResponse)
	defer srv.Close()

	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	expected := "Tesouro_SELIC_2027"
	if records[0].RecordKey != expected {
		t.Errorf("RecordKey = %q, want %q", records[0].RecordKey, expected)
	}
}

func TestTesouroDiretoCollector_EmptyResponse(t *testing.T) {
	srv := newTitulosServer(t, map[string]any{
		"response": map[string]any{
			"TrsrBdTradgList": []map[string]any{},
		},
	})
	defer srv.Close()

	c := tesouro.NewTesouroDiretoCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}
