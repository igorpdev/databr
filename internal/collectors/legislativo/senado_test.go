package legislativo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/legislativo"
)

var fakeSenadoResponse = map[string]any{
	"ListaParlamentarEmExercicio": map[string]any{
		"Parlamentares": map[string]any{
			"Parlamentar": []map[string]any{
				{
					"CodigoParlamentar": "5012",
					"NomeParlamentar":   "Test Senador",
					"SiglaPartido":      "MDB",
					"UfParlamentar":     "MG",
				},
				{
					"CodigoParlamentar": "5013",
					"NomeParlamentar":   "Outro Senador",
					"SiglaPartido":      "PT",
					"UfParlamentar":     "BA",
				},
			},
		},
	},
}

func newSenadoServer(t *testing.T, resp any, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestSenadoCollector_Source(t *testing.T) {
	srv := newSenadoServer(t, fakeSenadoResponse, http.StatusOK)
	defer srv.Close()

	c := legislativo.NewSenadoCollector(srv.URL)
	if got := c.Source(); got != "senado_senadores" {
		t.Errorf("Source() = %q, want %q", got, "senado_senadores")
	}
}

func TestSenadoCollector_Schedule(t *testing.T) {
	srv := newSenadoServer(t, fakeSenadoResponse, http.StatusOK)
	defer srv.Close()

	c := legislativo.NewSenadoCollector(srv.URL)
	if got := c.Schedule(); got != "0 22 * * 1-5" {
		t.Errorf("Schedule() = %q, want %q", got, "0 22 * * 1-5")
	}
}

func TestSenadoCollector_Collect(t *testing.T) {
	srv := newSenadoServer(t, fakeSenadoResponse, http.StatusOK)
	defer srv.Close()

	c := legislativo.NewSenadoCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "senado_senadores" {
		t.Errorf("Source = %q, want senado_senadores", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["NomeParlamentar"]; !ok {
		t.Error("Data must contain 'NomeParlamentar' field")
	}
	if _, ok := r.Data["CodigoParlamentar"]; !ok {
		t.Error("Data must contain 'CodigoParlamentar' field")
	}
}

func TestSenadoCollector_EmptyResponse(t *testing.T) {
	empty := map[string]any{
		"ListaParlamentarEmExercicio": map[string]any{
			"Parlamentares": map[string]any{
				"Parlamentar": []map[string]any{},
			},
		},
	}
	srv := newSenadoServer(t, empty, http.StatusOK)
	defer srv.Close()

	c := legislativo.NewSenadoCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestSenadoCollector_HTTPError(t *testing.T) {
	srv := newSenadoServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := legislativo.NewSenadoCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}
