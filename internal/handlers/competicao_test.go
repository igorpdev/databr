package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// mockIBGECNAEServer returns a test server that mimics the IBGE CNAE API.
// It responds to /divisoes/{code} with a JSON object containing id and descricao.
func mockIBGECNAEServer(divisions map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		for code, desc := range divisions {
			if strings.HasSuffix(path, "/divisoes/"+code) ||
				strings.HasSuffix(path, "/classes/"+code) ||
				strings.HasSuffix(path, "/grupos/"+code) ||
				strings.HasSuffix(path, "/subclasses/"+code) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"id":"%s","descricao":"%s"}`, code, desc)
				return
			}
		}
		http.NotFound(w, r)
	}))
}

func newCompeticaoRouter(h *handlers.CompeticaoHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/mercado/{cnae}/competicao", h.GetCompeticao)
	return r
}

func TestCompeticao_OK_WithStoreNil(t *testing.T) {
	// Mock IBGE CNAE API.
	ibgeSrv := mockIBGECNAEServer(map[string]string{
		"64": "Atividades de serviços financeiros",
	})
	defer ibgeSrv.Close()
	handlers.SetIBGECNAEBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGECNAEBaseURL("")

	h := handlers.NewCompeticaoHandler(nil) // store is nil
	router := newCompeticaoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/64/competicao", nil)
	req = x402pkg.InjectPrice(req, "0.030")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "competicao_setorial" {
		t.Errorf("Source = %q, want competicao_setorial", resp.Source)
	}
	if resp.CostUSDC != "0.030" {
		t.Errorf("CostUSDC = %q, want 0.030", resp.CostUSDC)
	}

	// Verify setor info is present from IBGE mock.
	setor, ok := resp.Data["setor"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[setor] to be a map")
	}
	if setor["codigo"] != "64" {
		t.Errorf("setor.codigo = %v, want 64", setor["codigo"])
	}
	if setor["descricao"] != "Atividades de serviços financeiros" {
		t.Errorf("setor.descricao = %v, want Atividades de serviços financeiros", setor["descricao"])
	}

	// Verify partial data indicators when store is nil.
	indicadores, ok := resp.Data["indicadores"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[indicadores] to be a map")
	}
	if indicadores["dados_parciais"] != true {
		t.Errorf("indicadores.dados_parciais = %v, want true", indicadores["dados_parciais"])
	}

	// Verify empty lists.
	emp, ok := resp.Data["empresas_listadas"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[empresas_listadas] to be a map")
	}
	total, _ := emp["total"].(float64)
	if total != 0 {
		t.Errorf("empresas_listadas.total = %v, want 0", total)
	}
}

func TestCompeticao_OK_WithStore(t *testing.T) {
	// Mock IBGE CNAE API.
	ibgeSrv := mockIBGECNAEServer(map[string]string{
		"64": "Financeiro",
	})
	defer ibgeSrv.Close()
	handlers.SetIBGECNAEBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGECNAEBaseURL("")

	now := time.Now()
	store := &ddStore{
		records: []domain.SourceRecord{
			// B3 record matching CNAE prefix "64"
			{Source: "b3_cotacoes", RecordKey: "ITUB4", Data: map[string]any{
				"ticker": "ITUB4", "cnae": "6421200", "nome": "Itau Unibanco",
			}, FetchedAt: now},
			// B3 record NOT matching CNAE prefix "64"
			{Source: "b3_cotacoes", RecordKey: "PETR4", Data: map[string]any{
				"ticker": "PETR4", "cnae": "0600001", "nome": "Petrobras",
			}, FetchedAt: now},
			// PNCP record with "Financeiro" in descricao (matches CNAE desc)
			{Source: "pncp_licitacoes", RecordKey: "lic-1", Data: map[string]any{
				"descricao": "Contratação de serviço Financeiro", "objeto": "consultoria",
			}, FetchedAt: now},
			// CVM fund with "Financeiro" in nome
			{Source: "cvm_fundos", RecordKey: "fund-1", Data: map[string]any{
				"nome": "Fundo Financeiro XP", "classe": "Renda Fixa",
			}, FetchedAt: now},
		},
	}

	h := handlers.NewCompeticaoHandler(store)
	router := newCompeticaoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/64/competicao", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Verify B3 filtering: only ITUB4 matches CNAE prefix "64".
	emp, ok := resp.Data["empresas_listadas"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[empresas_listadas] to be a map")
	}
	empTotal, _ := emp["total"].(float64)
	if empTotal != 1 {
		t.Errorf("empresas_listadas.total = %v, want 1", empTotal)
	}

	// Verify PNCP filtering: 1 match on "Financeiro".
	lic, ok := resp.Data["licitacoes_governo"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[licitacoes_governo] to be a map")
	}
	licTotal, _ := lic["total"].(float64)
	if licTotal != 1 {
		t.Errorf("licitacoes_governo.total = %v, want 1", licTotal)
	}

	// Verify CVM filtering: 1 match on "Financeiro".
	fundos, ok := resp.Data["fundos_exposicao"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[fundos_exposicao] to be a map")
	}
	fundosTotal, _ := fundos["total"].(float64)
	if fundosTotal != 1 {
		t.Errorf("fundos_exposicao.total = %v, want 1", fundosTotal)
	}

	// Verify indicadores.
	indicadores, ok := resp.Data["indicadores"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[indicadores] to be a map")
	}
	// HHI with 1 company: 100*100*1 = 10000
	hhi, _ := indicadores["hhi_estimado"].(float64)
	if hhi != 10000 {
		t.Errorf("hhi_estimado = %v, want 10000", hhi)
	}
	// Total activity = 1 + 1 + 1 = 3 → "baixa"
	atividade, _ := indicadores["atividade_mercado"].(string)
	if atividade != "baixa" {
		t.Errorf("atividade_mercado = %q, want baixa", atividade)
	}
}

func TestCompeticao_InvalidCNAE(t *testing.T) {
	h := handlers.NewCompeticaoHandler(nil)
	router := newCompeticaoRouter(h)

	tests := []struct {
		name string
		cnae string
	}{
		{"single_letter", "X"},
		{"single_digit", "1"},
		{"empty", ""},
		{"too_long", "12345678"},
		{"letters", "ABCD"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := "/v1/mercado/" + tc.cnae + "/competicao"
			if tc.cnae == "" {
				// Chi won't match empty param, so skip this case.
				return
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("cnae=%q: expected 400, got %d: %s", tc.cnae, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCompeticao_ValidCNAE2Digits(t *testing.T) {
	// Mock IBGE CNAE API.
	ibgeSrv := mockIBGECNAEServer(map[string]string{
		"64": "Atividades de serviços financeiros",
	})
	defer ibgeSrv.Close()
	handlers.SetIBGECNAEBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGECNAEBaseURL("")

	h := handlers.NewCompeticaoHandler(nil) // nil store is fine for this test
	router := newCompeticaoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/64/competicao", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid 2-digit CNAE, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	setor, ok := resp.Data["setor"].(map[string]any)
	if !ok {
		t.Fatal("expected Data[setor] to be a map")
	}
	if setor["descricao"] != "Atividades de serviços financeiros" {
		t.Errorf("setor.descricao = %v, want Atividades de serviços financeiros", setor["descricao"])
	}
}
