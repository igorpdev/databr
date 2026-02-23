package ibge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/ibge"
)

var fakePopulacaoResponse = []map[string]any{
	{
		"NC":  "1",
		"NN":  "Brasil",
		"MC":  "2",
		"MN":  "Unidade",
		"V":   "213317639",
		"D1C": "1",
		"D2C": "2025",
		"D2N": "2025",
		"D3C": "9324",
		"D3N": "Populacao estimada",
	},
	{
		"NC":  "11",
		"NN":  "Rondonia",
		"MC":  "2",
		"MN":  "Unidade",
		"V":   "1815278",
		"D1C": "1",
		"D2C": "2025",
		"D2N": "2025",
		"D3C": "9324",
		"D3N": "Populacao estimada",
	},
}

func newPopServer(t *testing.T, resp any, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestPopulacaoCollector_Source(t *testing.T) {
	srv := newPopServer(t, fakePopulacaoResponse, http.StatusOK)
	defer srv.Close()

	c := ibge.NewPopulacaoCollector(srv.URL)
	if got := c.Source(); got != "ibge_populacao" {
		t.Errorf("Source() = %q, want %q", got, "ibge_populacao")
	}
}

func TestPopulacaoCollector_Schedule(t *testing.T) {
	srv := newPopServer(t, fakePopulacaoResponse, http.StatusOK)
	defer srv.Close()

	c := ibge.NewPopulacaoCollector(srv.URL)
	if got := c.Schedule(); got != "0 8 1 * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 8 1 * *")
	}
}

func TestPopulacaoCollector_Collect(t *testing.T) {
	srv := newPopServer(t, fakePopulacaoResponse, http.StatusOK)
	defer srv.Close()

	c := ibge.NewPopulacaoCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "ibge_populacao" {
		t.Errorf("Source = %q, want ibge_populacao", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	for _, field := range []string{"localidade_id", "localidade_nome", "populacao", "periodo"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestPopulacaoCollector_EmptyResponse(t *testing.T) {
	srv := newPopServer(t, []map[string]any{}, http.StatusOK)
	defer srv.Close()

	c := ibge.NewPopulacaoCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestPopulacaoCollector_HTTPError(t *testing.T) {
	srv := newPopServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := ibge.NewPopulacaoCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}
