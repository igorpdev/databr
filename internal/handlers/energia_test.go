package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func newEnergiaRouter(h *handlers.EnergiaHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/energia/tarifas", h.GetTarifas)
	return r
}

func tarifaRecords() []domain.SourceRecord {
	return []domain.SourceRecord{
		{
			Source:    "aneel_tarifas",
			RecordKey: "53859112000169_2025-02-01_Tarifa de Aplicacao_B1_Convencional_Nao se aplica_MWh",
			Data: map[string]any{
				"distribuidora":        "CPFL JAGUARI",
				"cnpj":                 "53859112000169",
				"uf":                   "SP",
				"dat_inicio_vigencia":  "2025-02-01",
				"dat_fim_vigencia":     "2026-01-31",
				"base_tarifaria":       "Tarifa de Aplicacao",
				"subgrupo":             "B1",
				"modalidade_tarifaria": "Convencional",
				"classe":               "Residencial",
				"posto_tarifario":      "Nao se aplica",
				"unidade":              "MWh",
				"vlr_tusd":             "0,00",
				"vlr_te":               "999,00",
			},
			FetchedAt: time.Now(),
		},
		{
			Source:    "aneel_tarifas",
			RecordKey: "61695227000193_2025-01-01_Tarifa de Aplicacao_B1_Convencional_Nao se aplica_MWh",
			Data: map[string]any{
				"distribuidora":        "ENEL SP",
				"cnpj":                 "61695227000193",
				"uf":                   "SP",
				"dat_inicio_vigencia":  "2025-01-01",
				"dat_fim_vigencia":     "2025-12-31",
				"base_tarifaria":       "Tarifa de Aplicacao",
				"subgrupo":             "B1",
				"modalidade_tarifaria": "Convencional",
				"classe":               "Residencial",
				"posto_tarifario":      "Nao se aplica",
				"unidade":              "MWh",
				"vlr_tusd":             "0,00",
				"vlr_te":               "850,50",
			},
			FetchedAt: time.Now(),
		},
	}
}

func TestEnergiaHandler_GetTarifas_OK(t *testing.T) {
	store := &stubBCBStore{records: tarifaRecords()}
	h := handlers.NewEnergiaHandler(store)
	r := newEnergiaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/energia/tarifas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "aneel_tarifas" {
		t.Errorf("Source = %q, want aneel_tarifas", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	// The data field should contain a list of records.
	if resp.Data == nil {
		t.Error("expected non-nil Data field")
	}
}

func TestEnergiaHandler_GetTarifas_Empty(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewEnergiaHandler(store)
	r := newEnergiaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/energia/tarifas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no data, got %d", rec.Code)
	}
}

func TestEnergiaHandler_GetTarifas_FilterByDistribuidora(t *testing.T) {
	store := &stubBCBStore{records: tarifaRecords()}
	h := handlers.NewEnergiaHandler(store)
	r := newEnergiaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/energia/tarifas?distribuidora=CPFL+JAGUARI", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	records, ok := resp.Data["records"].([]any)
	if !ok {
		t.Fatalf("expected data.records to be []any, got %T", resp.Data["records"])
	}
	for _, item := range records {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		data, ok := m["data"].(map[string]any)
		if !ok {
			continue
		}
		if dist, ok := data["distribuidora"].(string); ok && dist != "CPFL JAGUARI" {
			t.Errorf("filter returned distribuidora %q, want CPFL JAGUARI", dist)
		}
	}
}

func TestEnergiaHandler_GetTarifas_FilterByDistribuidora_NotFound(t *testing.T) {
	store := &stubBCBStore{records: tarifaRecords()}
	h := handlers.NewEnergiaHandler(store)
	r := newEnergiaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/energia/tarifas?distribuidora=NAOEXISTE", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when filter matches nothing, got %d", rec.Code)
	}
}

func TestEnergiaHandler_GetTarifas_FilterByUF(t *testing.T) {
	store := &stubBCBStore{records: tarifaRecords()}
	h := handlers.NewEnergiaHandler(store)
	r := newEnergiaRouter(h)

	// Both fake records have uf=SP, so both should be returned.
	req := httptest.NewRequest(http.MethodGet, "/v1/energia/tarifas?uf=SP", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for uf=SP, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnergiaHandler_GetTarifas_FilterByUF_NotFound(t *testing.T) {
	store := &stubBCBStore{records: tarifaRecords()}
	h := handlers.NewEnergiaHandler(store)
	r := newEnergiaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/energia/tarifas?uf=AM", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown uf, got %d", rec.Code)
	}
}

func TestEnergiaHandler_GetTarifas_FormatContext(t *testing.T) {
	store := &stubBCBStore{records: tarifaRecords()}
	h := handlers.NewEnergiaHandler(store)
	r := newEnergiaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/energia/tarifas?format=context", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Context == "" {
		t.Error("expected non-empty Context when ?format=context")
	}
	if resp.CostUSDC != "0.002" {
		t.Errorf("expected cost 0.002 (+0.001 for context), got %s", resp.CostUSDC)
	}
}
