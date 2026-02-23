package juridico_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/juridico"
)

var fakeSTFResponse = map[string]any{
	"total": 2,
	"result": []map[string]any{
		{
			"id":              "ADI-1234",
			"nome":            "ADI 1234",
			"classe":          "ADI",
			"numero":          "1234",
			"relator":         "Min. Fulano de Tal",
			"orgao_julgador":  "Tribunal Pleno",
			"publicacao":      "2026-01-20",
			"julgamento":      "2026-01-15",
			"ementa":          "Ementa da decisao sobre inconstitucionalidade.",
		},
		{
			"id":              "RE-567890",
			"nome":            "RE 567890",
			"classe":          "RE",
			"numero":          "567890",
			"relator":         "Min. Ciclana Silva",
			"orgao_julgador":  "Segunda Turma",
			"publicacao":      "2026-01-22",
			"julgamento":      "2026-01-18",
			"ementa":          "Ementa sobre recurso extraordinario.",
		},
	},
}

func newSTFServer(t *testing.T, resp any, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestSTFCollector_Source(t *testing.T) {
	c := juridico.NewSTFCollector("http://localhost")
	if got := c.Source(); got != "stf_decisoes" {
		t.Errorf("Source() = %q, want %q", got, "stf_decisoes")
	}
}

func TestSTFCollector_Schedule(t *testing.T) {
	c := juridico.NewSTFCollector("http://localhost")
	if got := c.Schedule(); got != "0 13 * * 1-5" {
		t.Errorf("Schedule() = %q, want %q", got, "0 13 * * 1-5")
	}
}

func TestSTFCollector_Collect(t *testing.T) {
	srv := newSTFServer(t, fakeSTFResponse, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestSTFCollector_Collect_RecordKey(t *testing.T) {
	srv := newSTFServer(t, fakeSTFResponse, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if records[0].RecordKey != "ADI-1234" {
		t.Errorf("records[0].RecordKey = %q, want %q", records[0].RecordKey, "ADI-1234")
	}
	if records[1].RecordKey != "RE-567890" {
		t.Errorf("records[1].RecordKey = %q, want %q", records[1].RecordKey, "RE-567890")
	}
}

func TestSTFCollector_Collect_SourceField(t *testing.T) {
	srv := newSTFServer(t, fakeSTFResponse, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.Source != "stf_decisoes" {
			t.Errorf("Source = %q, want %q", rec.Source, "stf_decisoes")
		}
	}
}

func TestSTFCollector_Collect_DataFields(t *testing.T) {
	srv := newSTFServer(t, fakeSTFResponse, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one record")
	}

	rec := records[0]
	required := []string{
		"id", "nome", "classe", "numero", "relator",
		"orgao_julgador", "publicacao", "julgamento", "ementa",
	}
	for _, field := range required {
		if _, ok := rec.Data[field]; !ok {
			t.Errorf("Data must contain field %q", field)
		}
	}

	if got, _ := rec.Data["classe"].(string); got != "ADI" {
		t.Errorf("classe = %q, want ADI", got)
	}
	if got, _ := rec.Data["relator"].(string); got != "Min. Fulano de Tal" {
		t.Errorf("relator = %q, want Min. Fulano de Tal", got)
	}
}

func TestSTFCollector_Collect_EmptyResponse(t *testing.T) {
	emptyResp := map[string]any{
		"total":  0,
		"result": []any{},
	}
	srv := newSTFServer(t, emptyResp, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTFCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestSTFCollector_Collect_HTTPError(t *testing.T) {
	srv := newSTFServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := juridico.NewSTFCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}
