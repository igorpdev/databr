package legislativo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/legislativo"
)

var fakeCamaraResponse = map[string]any{
	"dados": []map[string]any{
		{"id": float64(204554), "nome": "Test Deputado", "siglaPartido": "PT", "siglaUf": "SP"},
		{"id": float64(204555), "nome": "Outro Deputado", "siglaPartido": "PL", "siglaUf": "RJ"},
	},
}

func newCamaraServer(t *testing.T, resp any, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestCamaraCollector_Source(t *testing.T) {
	srv := newCamaraServer(t, fakeCamaraResponse, http.StatusOK)
	defer srv.Close()

	c := legislativo.NewCamaraCollector(srv.URL)
	if got := c.Source(); got != "camara_deputados" {
		t.Errorf("Source() = %q, want %q", got, "camara_deputados")
	}
}

func TestCamaraCollector_Schedule(t *testing.T) {
	srv := newCamaraServer(t, fakeCamaraResponse, http.StatusOK)
	defer srv.Close()

	c := legislativo.NewCamaraCollector(srv.URL)
	if got := c.Schedule(); got != "0 22 * * 1-5" {
		t.Errorf("Schedule() = %q, want %q", got, "0 22 * * 1-5")
	}
}

func TestCamaraCollector_Collect(t *testing.T) {
	srv := newCamaraServer(t, fakeCamaraResponse, http.StatusOK)
	defer srv.Close()

	c := legislativo.NewCamaraCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "camara_deputados" {
		t.Errorf("Source = %q, want camara_deputados", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["nome"]; !ok {
		t.Error("Data must contain 'nome' field")
	}
	if _, ok := r.Data["siglaPartido"]; !ok {
		t.Error("Data must contain 'siglaPartido' field")
	}
	if _, ok := r.Data["siglaUf"]; !ok {
		t.Error("Data must contain 'siglaUf' field")
	}
}

func TestCamaraCollector_EmptyResponse(t *testing.T) {
	empty := map[string]any{"dados": []map[string]any{}}
	srv := newCamaraServer(t, empty, http.StatusOK)
	defer srv.Close()

	c := legislativo.NewCamaraCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestCamaraCollector_HTTPError(t *testing.T) {
	srv := newCamaraServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := legislativo.NewCamaraCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestCamaraCollector_RecordKeyUnique(t *testing.T) {
	srv := newCamaraServer(t, fakeCamaraResponse, http.StatusOK)
	defer srv.Close()

	c := legislativo.NewCamaraCollector(srv.URL)
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
