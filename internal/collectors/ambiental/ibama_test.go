package ambiental_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/ambiental"
)

var fakeIBAMAResponse = map[string]any{
	"total": 3,
	"data": []map[string]any{
		{
			"id":                  "EMB-001",
			"sei":                 "02001.000001/2026-01",
			"cpf_cnpj_infrator":   "12.345.678/0001-90",
			"nome_infrator":       "Fazenda Desmatamento Ltda",
			"municipio":           "Sao Felix do Xingu",
			"uf":                  "PA",
			"descricao_infracao":  "Desmatar, explorar economicamente ou degradar floresta",
			"data_embargo":        "2026-01-15",
			"area_embargada_ha":   150.5,
			"status":              "ATIVO",
			"bioma":               "Amazonia",
		},
		{
			"id":                  "EMB-002",
			"sei":                 "02001.000002/2026-01",
			"cpf_cnpj_infrator":   "98.765.432/0001-10",
			"nome_infrator":       "Agropecuaria Queimada SA",
			"municipio":           "Novo Progresso",
			"uf":                  "PA",
			"descricao_infracao":  "Fazer uso de fogo em areas agropastoris",
			"data_embargo":        "2026-01-20",
			"area_embargada_ha":   300.0,
			"status":              "ATIVO",
			"bioma":               "Amazonia",
		},
		{
			"id":                  "",
			"sei":                 "",
			"cpf_cnpj_infrator":   "11.111.111/0001-11",
			"nome_infrator":       "Sem Identificacao",
			"municipio":           "Altamira",
			"uf":                  "PA",
			"descricao_infracao":  "Infracao generica",
			"data_embargo":        "2026-01-25",
			"area_embargada_ha":   10.0,
			"status":              "CANCELADO",
			"bioma":               "Amazonia",
		},
	},
}

func newIBAMAServer(t *testing.T, resp any, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestIBAMACollector_Source(t *testing.T) {
	c := ambiental.NewIBAMACollector("http://localhost")
	if got := c.Source(); got != "ibama_embargos" {
		t.Errorf("Source() = %q, want %q", got, "ibama_embargos")
	}
}

func TestIBAMACollector_Schedule(t *testing.T) {
	c := ambiental.NewIBAMACollector("http://localhost")
	if got := c.Schedule(); got != "0 8 * * 1" {
		t.Errorf("Schedule() = %q, want %q", got, "0 8 * * 1")
	}
}

func TestIBAMACollector_Collect(t *testing.T) {
	srv := newIBAMAServer(t, fakeIBAMAResponse, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewIBAMACollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Expect 2 records (record with empty id+sei is skipped).
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestIBAMACollector_Collect_RecordKey(t *testing.T) {
	srv := newIBAMAServer(t, fakeIBAMAResponse, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewIBAMACollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if records[0].RecordKey != "EMB-001" {
		t.Errorf("records[0].RecordKey = %q, want %q", records[0].RecordKey, "EMB-001")
	}
	if records[1].RecordKey != "EMB-002" {
		t.Errorf("records[1].RecordKey = %q, want %q", records[1].RecordKey, "EMB-002")
	}
}

func TestIBAMACollector_Collect_SourceField(t *testing.T) {
	srv := newIBAMAServer(t, fakeIBAMAResponse, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewIBAMACollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.Source != "ibama_embargos" {
			t.Errorf("Source = %q, want %q", rec.Source, "ibama_embargos")
		}
	}
}

func TestIBAMACollector_Collect_DataFields(t *testing.T) {
	srv := newIBAMAServer(t, fakeIBAMAResponse, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewIBAMACollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one record")
	}

	rec := records[0]
	required := []string{
		"id", "sei", "cpf_cnpj_infrator", "nome_infrator",
		"municipio", "uf", "descricao_infracao", "data_embargo",
		"area_embargada_ha", "status", "bioma",
	}
	for _, field := range required {
		if _, ok := rec.Data[field]; !ok {
			t.Errorf("Data must contain field %q", field)
		}
	}

	if got, _ := rec.Data["municipio"].(string); got != "Sao Felix do Xingu" {
		t.Errorf("municipio = %q, want Sao Felix do Xingu", got)
	}
	if got, _ := rec.Data["bioma"].(string); got != "Amazonia" {
		t.Errorf("bioma = %q, want Amazonia", got)
	}
}

func TestIBAMACollector_Collect_SkipsEmptyID(t *testing.T) {
	srv := newIBAMAServer(t, fakeIBAMAResponse, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewIBAMACollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.RecordKey == "" {
			t.Error("found record with empty RecordKey; should have been skipped")
		}
	}
}

func TestIBAMACollector_Collect_EmptyResponse(t *testing.T) {
	emptyResp := map[string]any{
		"total": 0,
		"data":  []any{},
	}
	srv := newIBAMAServer(t, emptyResp, http.StatusOK)
	defer srv.Close()

	c := ambiental.NewIBAMACollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestIBAMACollector_Collect_HTTPError(t *testing.T) {
	srv := newIBAMAServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := ambiental.NewIBAMACollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}
