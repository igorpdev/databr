package tesouro_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/tesouro"
)

var fakeSICONFIResponse = map[string]any{
	"items": []map[string]any{
		{
			"cod_ibge":              4128559,
			"ente":                  "São Paulo",
			"uf":                    "SP",
			"esfera":                "E",
			"an_exercicio":          2024,
			"nr_periodo":            1,
			"co_tipo_demonstrativo": "RREO",
			"no_arquivo":            "RREO-Anexo01.csv",
		},
	},
}

func newSICONFIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeSICONFIResponse)
	}))
}

func TestSICONFICollector_Source(t *testing.T) {
	srv := newSICONFIServer(t)
	defer srv.Close()
	c := tesouro.NewSICONFICollector(srv.URL)
	if c.Source() != "tesouro_siconfi" {
		t.Errorf("Source() = %q, want tesouro_siconfi", c.Source())
	}
}

func TestSICONFICollector_FetchRREO(t *testing.T) {
	srv := newSICONFIServer(t)
	defer srv.Close()
	c := tesouro.NewSICONFICollector(srv.URL)
	records, err := c.FetchRREO(context.Background(), "SP", 2024, 1)
	if err != nil {
		t.Fatalf("FetchRREO() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].Source != "tesouro_siconfi" {
		t.Errorf("Source = %q, want tesouro_siconfi", records[0].Source)
	}
}
