package juridico_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/juridico"
)

var fakeDataJudResponse = map[string]any{
	"hits": map[string]any{
		"total": map[string]any{"value": 1},
		"hits": []map[string]any{
			{
				"_source": map[string]any{
					"numeroProcesso": "0001234-56.2023.8.26.0001",
					"tribunal":       "TJSP",
					"classe":         map[string]any{"nome": "Ação Civil Pública"},
				},
			},
		},
	},
}

func newDataJudServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeDataJudResponse)
	}))
}

func TestDataJudCollector_Source(t *testing.T) {
	srv := newDataJudServer(t)
	defer srv.Close()
	c := juridico.NewDataJudCollector(srv.URL, "test-key")
	if c.Source() != "datajud_cnj" {
		t.Errorf("Source() = %q, want datajud_cnj", c.Source())
	}
}

func TestDataJudCollector_Search(t *testing.T) {
	srv := newDataJudServer(t)
	defer srv.Close()
	c := juridico.NewDataJudCollector(srv.URL, "test-key")
	records, err := c.Search(context.Background(), "12345678909")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].Source != "datajud_cnj" {
		t.Errorf("Source = %q, want datajud_cnj", records[0].Source)
	}
}
