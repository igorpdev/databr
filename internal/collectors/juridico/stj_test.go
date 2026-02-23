package juridico_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/juridico"
)

var fakeSTJResponse = map[string]any{
	"total": 2,
	"result": []map[string]any{
		{
			"processo":        "REsp 1.234.567/SP",
			"classe":          "REsp",
			"numero":          "1234567",
			"relator":         "Min. Beltrano da Costa",
			"orgao_julgador":  "Terceira Turma",
			"data_julgamento": "2026-01-10",
			"data_publicacao": "2026-01-20",
			"ementa":          "Ementa sobre recurso especial em materia civil.",
			"acordao":         "Vistos, relatados e discutidos...",
		},
		{
			"processo":        "HC 987.654/RJ",
			"classe":          "HC",
			"numero":          "987654",
			"relator":         "Min. Sicrana Alves",
			"orgao_julgador":  "Sexta Turma",
			"data_julgamento": "2026-01-12",
			"data_publicacao": "2026-01-22",
			"ementa":          "Ementa sobre habeas corpus criminal.",
			"acordao":         "Acordao do habeas corpus...",
		},
	},
}

func newSTJServer(t *testing.T, resp any, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestSTJCollector_Source(t *testing.T) {
	c := juridico.NewSTJCollector("http://localhost")
	if got := c.Source(); got != "stj_decisoes" {
		t.Errorf("Source() = %q, want %q", got, "stj_decisoes")
	}
}

func TestSTJCollector_Schedule(t *testing.T) {
	c := juridico.NewSTJCollector("http://localhost")
	if got := c.Schedule(); got != "0 13 * * 1-5" {
		t.Errorf("Schedule() = %q, want %q", got, "0 13 * * 1-5")
	}
}

func TestSTJCollector_Collect(t *testing.T) {
	srv := newSTJServer(t, fakeSTJResponse, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTJCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestSTJCollector_Collect_RecordKey(t *testing.T) {
	srv := newSTJServer(t, fakeSTJResponse, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTJCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if records[0].RecordKey != "REsp 1.234.567/SP" {
		t.Errorf("records[0].RecordKey = %q, want %q", records[0].RecordKey, "REsp 1.234.567/SP")
	}
	if records[1].RecordKey != "HC 987.654/RJ" {
		t.Errorf("records[1].RecordKey = %q, want %q", records[1].RecordKey, "HC 987.654/RJ")
	}
}

func TestSTJCollector_Collect_SourceField(t *testing.T) {
	srv := newSTJServer(t, fakeSTJResponse, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTJCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.Source != "stj_decisoes" {
			t.Errorf("Source = %q, want %q", rec.Source, "stj_decisoes")
		}
	}
}

func TestSTJCollector_Collect_DataFields(t *testing.T) {
	srv := newSTJServer(t, fakeSTJResponse, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTJCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one record")
	}

	rec := records[0]
	required := []string{
		"processo", "classe", "numero", "relator",
		"orgao_julgador", "data_julgamento", "data_publicacao",
		"ementa", "acordao",
	}
	for _, field := range required {
		if _, ok := rec.Data[field]; !ok {
			t.Errorf("Data must contain field %q", field)
		}
	}

	if got, _ := rec.Data["classe"].(string); got != "REsp" {
		t.Errorf("classe = %q, want REsp", got)
	}
	if got, _ := rec.Data["relator"].(string); got != "Min. Beltrano da Costa" {
		t.Errorf("relator = %q, want Min. Beltrano da Costa", got)
	}
}

func TestSTJCollector_Collect_EmptyResponse(t *testing.T) {
	emptyResp := map[string]any{
		"total":  0,
		"result": []any{},
	}
	srv := newSTJServer(t, emptyResp, http.StatusOK)
	defer srv.Close()

	c := juridico.NewSTJCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestSTJCollector_Collect_HTTPError(t *testing.T) {
	srv := newSTJServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := juridico.NewSTJCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}
