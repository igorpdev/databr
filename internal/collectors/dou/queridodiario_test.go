package dou_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/dou"
)

var fakeGazetteResponse = map[string]any{
	"gazettes": []map[string]any{
		{
			"territory_id":   "3550308",
			"territory_name": "São Paulo",
			"state_code":     "SP",
			"date":           "2026-02-01",
			"url":            "https://queridodiario.ok.org.br/diario",
			"excerpts":       []string{"contrato de licitação..."},
		},
	},
	"total_gazettes": 1,
}

func newQDServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeGazetteResponse)
	}))
}

func TestQDCollector_Source(t *testing.T) {
	srv := newQDServer(t)
	defer srv.Close()
	c := dou.NewQDCollector(srv.URL)
	if c.Source() != "querido_diario" {
		t.Errorf("Source() = %q, want querido_diario", c.Source())
	}
}

func TestQDCollector_Search(t *testing.T) {
	srv := newQDServer(t)
	defer srv.Close()
	c := dou.NewQDCollector(srv.URL)
	records, err := c.Search(context.Background(), dou.SearchParams{Query: "contrato"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", records[0].Source)
	}
}
