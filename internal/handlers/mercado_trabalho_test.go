package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// mockIBGEPNADServer returns a test server that mimics the IBGE SIDRA API
// for PNAD/population data. It responds to paths containing the UF IBGE code.
func mockIBGEPNADServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a valid JSON array response for any request.
		fmt.Fprint(w, `[{"id":"9324","variavel":"Populacao","resultados":[{"classificacoes":[],"series":[{"localidade":{"id":"35","nivel":{"id":"N3","nome":"Unidade da Federacao"},"nome":"Sao Paulo"},"serie":{"2024":"44000000"}}]}]}]`)
	}))
}

func newMercadoTrabalhoRouter(h *handlers.MercadoTrabalhoHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/mercado-trabalho/{uf}/analise", h.GetMercadoTrabalho)
	return r
}

func TestMercadoTrabalho_OK_NoStore(t *testing.T) {
	// Mock IBGE PNAD API.
	ibgeSrv := mockIBGEPNADServer()
	defer ibgeSrv.Close()
	handlers.SetIBGEPNADBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGEPNADBaseURL("")

	h := handlers.NewMercadoTrabalhoHandler(nil) // store is nil
	router := newMercadoTrabalhoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado-trabalho/SP/analise", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "mercado_trabalho" {
		t.Errorf("Source = %q, want mercado_trabalho", resp.Source)
	}
	if resp.CostUSDC != "0.010" {
		t.Errorf("CostUSDC = %q, want 0.010", resp.CostUSDC)
	}

	// Verify estado section.
	estado, ok := resp.Data["estado"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[estado] to be a map")
	}
	if estado["uf"] != "SP" {
		t.Errorf("estado.uf = %v, want SP", estado["uf"])
	}
	if estado["nome"] != "São Paulo" {
		t.Errorf("estado.nome = %v, want São Paulo", estado["nome"])
	}

	// Verify demographic data is present (from IBGE mock).
	demo, ok := resp.Data["demografia"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[demografia] to be a map")
	}
	if demo["disponivel"] != true {
		t.Errorf("demografia.disponivel = %v, want true", demo["disponivel"])
	}

	// Verify emprego section indicates store not available.
	emprego, ok := resp.Data["emprego"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[emprego] to be a map")
	}
	if emprego["disponivel"] != false {
		t.Errorf("emprego.disponivel = %v, want false", emprego["disponivel"])
	}

	// Verify tendencia is indeterminada when store is nil.
	if resp.Data["tendencia"] != "indeterminada" {
		t.Errorf("tendencia = %v, want indeterminada", resp.Data["tendencia"])
	}
}

func TestMercadoTrabalho_OK_WithCAGED(t *testing.T) {
	// Mock IBGE PNAD API.
	ibgeSrv := mockIBGEPNADServer()
	defer ibgeSrv.Close()
	handlers.SetIBGEPNADBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGEPNADBaseURL("")

	now := time.Now()
	store := &ddStore{
		records: []domain.SourceRecord{
			// CAGED records for SP with positive saldo.
			{Source: "caged_emprego", RecordKey: "caged-sp-1", Data: map[string]any{
				"uf": "SP", "admissoes": float64(1000), "desligamentos": float64(800), "saldo": float64(200), "setor": "Tecnologia",
			}, FetchedAt: now},
			{Source: "caged_emprego", RecordKey: "caged-sp-2", Data: map[string]any{
				"uf": "SP", "admissoes": float64(500), "desligamentos": float64(300), "saldo": float64(200), "setor": "Financeiro",
			}, FetchedAt: now},
			// PIB record.
			{Source: "ibge_pib", RecordKey: "pib-latest", Data: map[string]any{
				"valor": "2.9", "periodo": "2025-Q3",
			}, FetchedAt: now},
			// IPCA record.
			{Source: "ibge_ipca", RecordKey: "ipca-latest", Data: map[string]any{
				"valor": "4.56", "periodo": "202501",
			}, FetchedAt: now},
		},
	}

	h := handlers.NewMercadoTrabalhoHandler(store)
	router := newMercadoTrabalhoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado-trabalho/SP/analise", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Verify emprego data is available.
	emprego, ok := resp.Data["emprego"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[emprego] to be a map")
	}
	if emprego["disponivel"] != true {
		t.Errorf("emprego.disponivel = %v, want true", emprego["disponivel"])
	}

	// Verify CAGED data is present.
	caged, ok := emprego["caged"].(map[string]any)
	if !ok {
		t.Fatal("expected emprego[caged] to be a map")
	}
	admissoes, _ := caged["admissoes"].(float64)
	if admissoes != 1500 {
		t.Errorf("caged.admissoes = %v, want 1500", admissoes)
	}
	desligamentos, _ := caged["desligamentos"].(float64)
	if desligamentos != 1100 {
		t.Errorf("caged.desligamentos = %v, want 1100", desligamentos)
	}
	saldo, _ := caged["saldo"].(float64)
	if saldo != 400 {
		t.Errorf("caged.saldo = %v, want 400", saldo)
	}

	// Verify tendencia is "crescimento" (positive saldo).
	if resp.Data["tendencia"] != "crescimento" {
		t.Errorf("tendencia = %v, want crescimento", resp.Data["tendencia"])
	}

	// Verify economic indicators are present.
	indicadores, ok := resp.Data["indicadores_economicos"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[indicadores_economicos] to be a map")
	}
	if indicadores["disponivel"] != true {
		t.Errorf("indicadores_economicos.disponivel = %v, want true", indicadores["disponivel"])
	}
	if indicadores["pib"] == nil {
		t.Error("expected indicadores_economicos[pib] to be present")
	}
	if indicadores["ipca"] == nil {
		t.Error("expected indicadores_economicos[ipca] to be present")
	}
}

func TestMercadoTrabalho_InvalidUF(t *testing.T) {
	h := handlers.NewMercadoTrabalhoHandler(nil)
	router := newMercadoTrabalhoRouter(h)

	tests := []struct {
		name string
		uf   string
	}{
		{"invalid_XX", "XX"},
		{"invalid_ZZ", "ZZ"},
		{"invalid_BR", "BR"},
		{"three_chars", "SPP"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/mercado-trabalho/"+tc.uf+"/analise", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("uf=%q: expected 400, got %d: %s", tc.uf, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMercadoTrabalho_ValidUF(t *testing.T) {
	// Mock IBGE PNAD API.
	ibgeSrv := mockIBGEPNADServer()
	defer ibgeSrv.Close()
	handlers.SetIBGEPNADBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGEPNADBaseURL("")

	h := handlers.NewMercadoTrabalhoHandler(nil)
	router := newMercadoTrabalhoRouter(h)

	// Test with lowercase "sp" — handler should normalize to "SP".
	req := httptest.NewRequest(http.MethodGet, "/v1/mercado-trabalho/sp/analise", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid UF 'sp', got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	estado, ok := resp.Data["estado"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[estado] to be a map")
	}
	// Should be uppercased.
	if estado["uf"] != "SP" {
		t.Errorf("estado.uf = %v, want SP", estado["uf"])
	}
	if estado["nome"] != "São Paulo" {
		t.Errorf("estado.nome = %v, want São Paulo", estado["nome"])
	}
}
