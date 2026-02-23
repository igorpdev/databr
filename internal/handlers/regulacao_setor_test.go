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

func newRegulacaoSetorRouter(h *handlers.RegulacaoSetorHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/setor/{cnae}/regulacao", h.GetRegulacaoSetor)
	return r
}

func TestRegulacaoSetor_OK_FinancialSector(t *testing.T) {
	// Mock IBGE CNAE API.
	ibgeSrv := mockIBGECNAEServer(map[string]string{
		"64": "Atividades de serviços financeiros",
	})
	defer ibgeSrv.Close()
	handlers.SetIBGECNAEBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGECNAEBaseURL("")

	now := time.Now()
	store := &ddStore{
		records: []domain.SourceRecord{
			{Source: "camara_deputados", RecordKey: "pl-1", Data: map[string]any{
				"ementa": "Regula atividades de serviços financeiros", "numero": "PL 1234/2025",
			}, FetchedAt: now},
			{Source: "cgu_compliance", RecordKey: "sanc-1", Data: map[string]any{
				"tipo": "CEIS", "entidade": "Empresa X",
			}, FetchedAt: now},
		},
	}

	h := handlers.NewRegulacaoSetorHandler(store)
	router := newRegulacaoSetorRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/setor/64/regulacao", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "regulacao_setorial" {
		t.Errorf("Source = %q, want regulacao_setorial", resp.Source)
	}
	if resp.CostUSDC != "0.015" {
		t.Errorf("CostUSDC = %q, want 0.015", resp.CostUSDC)
	}

	// Verify nivel_regulacao is "alto" for financial sector (CNAE 64).
	nivelRegulacao, _ := resp.Data["nivel_regulacao"].(string)
	if nivelRegulacao != "alto" {
		t.Errorf("nivel_regulacao = %q, want alto", nivelRegulacao)
	}

	// Verify reguladores contains BCB, CVM, and SUSEP.
	reguladores, ok := resp.Data["reguladores_principais"].([]any)
	if !ok {
		t.Fatal("expected Data[reguladores_principais] to be an array")
	}
	if len(reguladores) != 3 {
		t.Fatalf("expected 3 reguladores for CNAE 64, got %d", len(reguladores))
	}

	// Check that BCB, CVM, and SUSEP are present.
	siglas := map[string]bool{}
	for _, reg := range reguladores {
		regMap, ok := reg.(map[string]any)
		if !ok {
			t.Fatal("expected regulador to be a map")
		}
		sigla, _ := regMap["sigla"].(string)
		siglas[sigla] = true
	}
	for _, expected := range []string{"BCB", "CVM", "SUSEP"} {
		if !siglas[expected] {
			t.Errorf("expected regulador %q to be present", expected)
		}
	}

	// Verify CNAE info.
	cnaeInfo, ok := resp.Data["cnae"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[cnae] to be a map")
	}
	if cnaeInfo["codigo"] != "64" {
		t.Errorf("cnae.codigo = %v, want 64", cnaeInfo["codigo"])
	}
	if cnaeInfo["descricao"] != "Atividades de serviços financeiros" {
		t.Errorf("cnae.descricao = %v, want Atividades de serviços financeiros", cnaeInfo["descricao"])
	}

	// Verify compliance requirements include financial-sector-specific entries.
	requisitos, ok := resp.Data["requisitos_compliance"].([]any)
	if !ok {
		t.Fatal("expected Data[requisitos_compliance] to be an array")
	}
	// Should have at least the 2 general + 3 financial + 1 monitoring = 6.
	if len(requisitos) < 5 {
		t.Errorf("expected at least 5 requisitos for financial sector, got %d", len(requisitos))
	}

	// Verify legislacao_recente includes the matching law.
	legislacao, ok := resp.Data["legislacao_recente"].([]any)
	if !ok {
		t.Fatal("expected Data[legislacao_recente] to be an array")
	}
	if len(legislacao) != 1 {
		t.Errorf("expected 1 legislacao entry matching 'serviços financeiros', got %d", len(legislacao))
	}
}

func TestRegulacaoSetor_OK_NoStore(t *testing.T) {
	// Mock IBGE CNAE API.
	ibgeSrv := mockIBGECNAEServer(map[string]string{
		"64": "Atividades de serviços financeiros",
	})
	defer ibgeSrv.Close()
	handlers.SetIBGECNAEBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGECNAEBaseURL("")

	h := handlers.NewRegulacaoSetorHandler(nil) // store is nil
	router := newRegulacaoSetorRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/setor/64/regulacao", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "regulacao_setorial" {
		t.Errorf("Source = %q, want regulacao_setorial", resp.Source)
	}

	// Verify legislacao_recente is empty when store is nil.
	legislacao, ok := resp.Data["legislacao_recente"].([]any)
	if !ok {
		t.Fatal("expected Data[legislacao_recente] to be an array")
	}
	if len(legislacao) != 0 {
		t.Errorf("expected 0 legislacao entries with nil store, got %d", len(legislacao))
	}

	// Reguladores should still be present (static mapping).
	reguladores, ok := resp.Data["reguladores_principais"].([]any)
	if !ok {
		t.Fatal("expected Data[reguladores_principais] to be an array")
	}
	if len(reguladores) != 3 {
		t.Errorf("expected 3 reguladores for CNAE 64 (BCB, CVM, SUSEP), got %d", len(reguladores))
	}

	// nivel_regulacao should still be "alto".
	nivelRegulacao, _ := resp.Data["nivel_regulacao"].(string)
	if nivelRegulacao != "alto" {
		t.Errorf("nivel_regulacao = %q, want alto", nivelRegulacao)
	}
}

func TestRegulacaoSetor_InvalidCNAE(t *testing.T) {
	h := handlers.NewRegulacaoSetorHandler(nil)
	router := newRegulacaoSetorRouter(h)

	tests := []struct {
		name string
		cnae string
	}{
		{"single_letter", "X"},
		{"single_digit", "1"},
		{"too_long", "12345678"},
		{"letters", "ABCD"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/setor/"+tc.cnae+"/regulacao", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("cnae=%q: expected 400, got %d: %s", tc.cnae, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRegulacaoSetor_UnregulatedSector(t *testing.T) {
	// Mock IBGE CNAE API for retail (CNAE 47).
	ibgeSrv := mockIBGECNAEServer(map[string]string{
		"47": "Comércio varejista",
	})
	defer ibgeSrv.Close()
	handlers.SetIBGECNAEBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGECNAEBaseURL("")

	h := handlers.NewRegulacaoSetorHandler(nil) // nil store
	router := newRegulacaoSetorRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/setor/47/regulacao", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Verify nivel_regulacao is "baixo" for retail (CNAE 47).
	nivelRegulacao, _ := resp.Data["nivel_regulacao"].(string)
	if nivelRegulacao != "baixo" {
		t.Errorf("nivel_regulacao = %q, want baixo", nivelRegulacao)
	}

	// Verify no reguladores for retail.
	reguladores, ok := resp.Data["reguladores_principais"].([]any)
	if !ok {
		t.Fatal("expected Data[reguladores_principais] to be an array")
	}
	if len(reguladores) != 0 {
		t.Errorf("expected 0 reguladores for unregulated CNAE 47, got %d", len(reguladores))
	}

	// Verify CNAE info.
	cnaeInfo, ok := resp.Data["cnae"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[cnae] to be a map")
	}
	if cnaeInfo["descricao"] != "Comércio varejista" {
		t.Errorf("cnae.descricao = %v, want Comércio varejista", cnaeInfo["descricao"])
	}

	// Verify only general requirements (no sector-specific ones).
	requisitos, ok := resp.Data["requisitos_compliance"].([]any)
	if !ok {
		t.Fatal("expected Data[requisitos_compliance] to be an array")
	}
	if len(requisitos) != 2 {
		t.Errorf("expected exactly 2 general requisitos for unregulated sector, got %d", len(requisitos))
	}
}
