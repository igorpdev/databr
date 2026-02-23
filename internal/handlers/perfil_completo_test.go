package handlers_test

import (
	"encoding/json"
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

// ---------------------------------------------------------------------------
// Router helpers
// ---------------------------------------------------------------------------

func newPerfilCompletoRouter(h *handlers.PerfilCompletoHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/empresas/{cnpj}/perfil-completo", h.GetPerfilCompleto)
	return r
}

func newCarteiraRiscoRouter(h *handlers.CarteiraRiscoHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/v1/carteira/risco", h.PostCarteiraRisco)
	return r
}

// ---------------------------------------------------------------------------
// Perfil Completo tests
// ---------------------------------------------------------------------------

func TestPerfilCompleto_OK_Clean(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":                 "12345678000195",
				"razao_social":         "EMPRESA XPTO LTDA",
				"situacao_cadastral":   "ATIVA",
			},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{records: nil}
	judicialSearcher := &ddJudicialSearcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewPerfilCompletoHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newPerfilCompletoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/perfil-completo", nil)
	req = x402pkg.InjectPrice(req, "0.020")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "perfil_completo" {
		t.Errorf("Source = %q, want perfil_completo", resp.Source)
	}
	if resp.CostUSDC != "0.020" {
		t.Errorf("CostUSDC = %q, want 0.020", resp.CostUSDC)
	}

	riskScore, _ := resp.Data["risk_score"].(float64)
	if riskScore != 0 {
		t.Errorf("risk_score = %v, want 0 for clean company", riskScore)
	}

	cadastro, _ := resp.Data["cadastro"].(map[string]any)
	if cadastro["status"] != "limpo" {
		t.Errorf("cadastro.status = %q, want limpo", cadastro["status"])
	}

	compliance, _ := resp.Data["compliance"].(map[string]any)
	if compliance["status"] != "limpo" {
		t.Errorf("compliance.status = %q, want limpo", compliance["status"])
	}

	judicial, _ := resp.Data["judicial"].(map[string]any)
	if judicial["status"] != "limpo" {
		t.Errorf("judicial.status = %q, want limpo", judicial["status"])
	}

	ambiental, _ := resp.Data["ambiental"].(map[string]any)
	if ambiental["status"] != "limpo" {
		t.Errorf("ambiental.status = %q, want limpo", ambiental["status"])
	}

	contratosGov, _ := resp.Data["contratos_governo"].(map[string]any)
	if contratosGov["status"] != "limpo" {
		t.Errorf("contratos_governo.status = %q, want limpo", contratosGov["status"])
	}
}

func TestPerfilCompleto_HighRisk(t *testing.T) {
	// No company data → +15 (no cadastro) + +5 (alerta status)
	cnpjFetcher := &ddCNPJFetcher{records: nil}

	// Compliance sanctions → +30
	complianceFetcher := &ddComplianceFetcher{
		records: []domain.SourceRecord{{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}},
	}

	// 10+ judicial processes → 10*3 = 30 (cap is 30)
	judicialRecords := make([]domain.SourceRecord, 12)
	for i := range judicialRecords {
		judicialRecords[i] = domain.SourceRecord{
			Source: "datajud_cnj",
			Data:   map[string]any{"processo": i + 1},
		}
	}
	judicialSearcher := &ddJudicialSearcher{records: judicialRecords}

	// IBAMA embargos → +25
	store := &ddStore{
		records: []domain.SourceRecord{{
			Source: "ibama_embargos",
			Data:   map[string]any{"cpf_cnpj": "12345678000195", "embargo": "area protegida"},
		}},
	}

	h := handlers.NewPerfilCompletoHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newPerfilCompletoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/perfil-completo", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	riskScore, _ := resp.Data["risk_score"].(float64)
	// 15 (no cadastro) + 5 (alerta) + 30 (compliance) + 30 (judicial capped) + 25 (ambiental) = 105 → capped at 100
	if riskScore != 100 {
		t.Errorf("risk_score = %v, want 100 (capped)", riskScore)
	}

	compliance, _ := resp.Data["compliance"].(map[string]any)
	if compliance["status"] != "critico" {
		t.Errorf("compliance.status = %q, want critico", compliance["status"])
	}

	judicial, _ := resp.Data["judicial"].(map[string]any)
	if judicial["status"] != "critico" {
		t.Errorf("judicial.status = %q, want critico (12 processes >= 10)", judicial["status"])
	}

	ambiental, _ := resp.Data["ambiental"].(map[string]any)
	if ambiental["status"] != "critico" {
		t.Errorf("ambiental.status = %q, want critico", ambiental["status"])
	}
}

func TestPerfilCompleto_InvalidCNPJ(t *testing.T) {
	h := handlers.NewPerfilCompletoHandler(
		&ddCNPJFetcher{},
		&ddComplianceFetcher{},
		&ddJudicialSearcher{},
		&ddStore{},
	)
	router := newPerfilCompletoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/123/perfil-completo", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPerfilCompleto_InactiveCompany(t *testing.T) {
	// Company found but situacao_cadastral is "SUSPENSA" → cadastro alerta → +5
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":               "12345678000195",
				"razao_social":       "EMPRESA SUSPENSA SA",
				"situacao_cadastral": "SUSPENSA",
			},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{records: nil}
	judicialSearcher := &ddJudicialSearcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewPerfilCompletoHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newPerfilCompletoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/perfil-completo", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	cadastro, _ := resp.Data["cadastro"].(map[string]any)
	if cadastro["status"] != "alerta" {
		t.Errorf("cadastro.status = %q, want alerta for inactive company", cadastro["status"])
	}

	riskScore, _ := resp.Data["risk_score"].(float64)
	// Company found but inactive: only +5 (alerta status, no +15 since data was retrieved)
	if riskScore != 5 {
		t.Errorf("risk_score = %v, want 5", riskScore)
	}
}

func TestPerfilCompleto_RiskScoreCap100(t *testing.T) {
	// All risk factors maxed out to exceed 100, verify cap.
	// No company data → +15 + +5 (alerta)
	cnpjFetcher := &ddCNPJFetcher{records: nil}

	// Sanctions → +30
	complianceFetcher := &ddComplianceFetcher{
		records: []domain.SourceRecord{
			{Source: "cgu_compliance", Data: map[string]any{"sanction": "CEIS"}},
			{Source: "cgu_compliance", Data: map[string]any{"sanction": "CNEP"}},
		},
	}

	// 20 judicial processes → 20*3 = 60, capped at 30
	judicialRecords := make([]domain.SourceRecord, 20)
	for i := range judicialRecords {
		judicialRecords[i] = domain.SourceRecord{
			Source: "datajud_cnj",
			Data:   map[string]any{"processo": i + 1},
		}
	}
	judicialSearcher := &ddJudicialSearcher{records: judicialRecords}

	// IBAMA embargos → +25
	store := &ddStore{
		records: []domain.SourceRecord{{
			Source: "ibama_embargos",
			Data:   map[string]any{"cpf_cnpj": "12345678000195", "embargo": "desmatamento"},
		}},
	}

	h := handlers.NewPerfilCompletoHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newPerfilCompletoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/perfil-completo", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	riskScore, _ := resp.Data["risk_score"].(float64)
	// 15 + 5 + 30 + 30 + 25 = 105 → capped at 100
	if riskScore != 100 {
		t.Errorf("risk_score = %v, want 100 (capped at max)", riskScore)
	}
}

// ---------------------------------------------------------------------------
// Carteira Risco tests
// ---------------------------------------------------------------------------

func TestCarteiraRisco_OK_SingleCNPJ(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":                 "12345678000195",
				"razao_social":         "EMPRESA LIMPA SA",
				"situacao_cadastral":   "ATIVA",
				"uf":                   "SP",
			},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewCarteiraRiscoHandler(cnpjFetcher, complianceFetcher, store)
	router := newCarteiraRiscoRouter(h)

	body := `{"cnpjs":["12345678000195"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/carteira/risco", strings.NewReader(body))
	req = x402pkg.InjectPrice(req, "0.150")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "carteira_risco" {
		t.Errorf("Source = %q, want carteira_risco", resp.Source)
	}
	if resp.CostUSDC != "0.150" {
		t.Errorf("CostUSDC = %q, want 0.150", resp.CostUSDC)
	}

	totalCNPJs, _ := resp.Data["total_cnpjs"].(float64)
	if totalCNPJs != 1 {
		t.Errorf("total_cnpjs = %v, want 1", totalCNPJs)
	}

	riscoMedio, _ := resp.Data["risco_medio"].(float64)
	if riscoMedio != 0 {
		t.Errorf("risco_medio = %v, want 0 for clean company", riscoMedio)
	}

	nivelRiscoMedio, _ := resp.Data["nivel_risco_medio"].(string)
	if nivelRiscoMedio != "baixo" {
		t.Errorf("nivel_risco_medio = %q, want baixo", nivelRiscoMedio)
	}

	// Check the empresas array.
	empresasRaw, ok := resp.Data["empresas"]
	if !ok {
		t.Fatal("missing empresas in response")
	}
	empresas, ok := empresasRaw.([]any)
	if !ok || len(empresas) != 1 {
		t.Fatalf("empresas length = %d, want 1", len(empresas))
	}
	emp, _ := empresas[0].(map[string]any)
	if emp["cnpj"] != "12345678000195" {
		t.Errorf("empresa cnpj = %q, want 12345678000195", emp["cnpj"])
	}
	empScore, _ := emp["risk_score"].(float64)
	if empScore != 0 {
		t.Errorf("empresa risk_score = %v, want 0", empScore)
	}
	if emp["risk_level"] != "baixo" {
		t.Errorf("empresa risk_level = %q, want baixo", emp["risk_level"])
	}
}

func TestCarteiraRisco_EmptyBody(t *testing.T) {
	h := handlers.NewCarteiraRiscoHandler(
		&ddCNPJFetcher{},
		&ddComplianceFetcher{},
		&ddStore{},
	)
	router := newCarteiraRiscoRouter(h)

	body := `{"cnpjs":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/carteira/risco", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCarteiraRisco_InvalidJSON(t *testing.T) {
	h := handlers.NewCarteiraRiscoHandler(
		&ddCNPJFetcher{},
		&ddComplianceFetcher{},
		&ddStore{},
	)
	router := newCarteiraRiscoRouter(h)

	body := `{this is not valid json}`
	req := httptest.NewRequest(http.MethodPost, "/v1/carteira/risco", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCarteiraRisco_ExceedsMaxCNPJs(t *testing.T) {
	h := handlers.NewCarteiraRiscoHandler(
		&ddCNPJFetcher{},
		&ddComplianceFetcher{},
		&ddStore{},
	)
	router := newCarteiraRiscoRouter(h)

	// Build 51 valid CNPJs (all the same, just to exceed the limit).
	cnpjs := make([]string, 51)
	for i := range cnpjs {
		cnpjs[i] = "12345678000195"
	}
	bodyBytes, _ := json.Marshal(map[string]any{"cnpjs": cnpjs})

	req := httptest.NewRequest(http.MethodPost, "/v1/carteira/risco", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCarteiraRisco_InvalidCNPJ(t *testing.T) {
	h := handlers.NewCarteiraRiscoHandler(
		&ddCNPJFetcher{},
		&ddComplianceFetcher{},
		&ddStore{},
	)
	router := newCarteiraRiscoRouter(h)

	body := `{"cnpjs":["12345678000195","999"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/carteira/risco", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCarteiraRisco_PortfolioSummary(t *testing.T) {
	// We use 3 CNPJs — all the same valid CNPJ (12345678000195) since the mock
	// returns the same records regardless. The handler treats each independently.
	//
	// Mock setup: company found (ATIVA) + sanctioned + no judicial from store.
	// Per-CNPJ score: 0 (company ok) + 35 (sanctioned) = 35 each → "medio"
	// Average: 35 → "medio"
	// Distribution: 3 medio
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":               "12345678000195",
				"razao_social":       "EMPRESA TESTE SA",
				"situacao_cadastral": "ATIVA",
				"uf":                 "RJ",
				"cnae_fiscal":        "6201500",
			},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{
		records: []domain.SourceRecord{{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}},
	}
	store := &ddStore{records: nil} // no judicial records

	h := handlers.NewCarteiraRiscoHandler(cnpjFetcher, complianceFetcher, store)
	router := newCarteiraRiscoRouter(h)

	body := `{"cnpjs":["12345678000195","12345678000195","12345678000195"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/carteira/risco", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	totalCNPJs, _ := resp.Data["total_cnpjs"].(float64)
	if totalCNPJs != 3 {
		t.Errorf("total_cnpjs = %v, want 3", totalCNPJs)
	}

	riscoMedio, _ := resp.Data["risco_medio"].(float64)
	// Each CNPJ: 0 (active company) + 35 (sanctioned) = 35; avg = 35
	if riscoMedio != 35 {
		t.Errorf("risco_medio = %v, want 35", riscoMedio)
	}

	nivelRiscoMedio, _ := resp.Data["nivel_risco_medio"].(string)
	if nivelRiscoMedio != "medio" {
		t.Errorf("nivel_risco_medio = %q, want medio", nivelRiscoMedio)
	}

	// Check risk distribution.
	dist, _ := resp.Data["distribuicao_risco"].(map[string]any)
	baixo, _ := dist["baixo"].(float64)
	medio, _ := dist["medio"].(float64)
	alto, _ := dist["alto"].(float64)
	critico, _ := dist["critico"].(float64)

	if baixo != 0 {
		t.Errorf("distribution baixo = %v, want 0", baixo)
	}
	if medio != 3 {
		t.Errorf("distribution medio = %v, want 3", medio)
	}
	if alto != 0 {
		t.Errorf("distribution alto = %v, want 0", alto)
	}
	if critico != 0 {
		t.Errorf("distribution critico = %v, want 0", critico)
	}

	// Check geographic concentration.
	geo, _ := resp.Data["concentracao_geografica"].(map[string]any)
	rjCount, _ := geo["RJ"].(float64)
	if rjCount != 3 {
		t.Errorf("geographic RJ = %v, want 3", rjCount)
	}

	// Check sector concentration.
	setor, _ := resp.Data["concentracao_setorial"].(map[string]any)
	cnaeCount, _ := setor["6201500"].(float64)
	if cnaeCount != 3 {
		t.Errorf("sector 6201500 = %v, want 3", cnaeCount)
	}

	// Verify empresas array entries.
	empresasRaw, _ := resp.Data["empresas"].([]any)
	if len(empresasRaw) != 3 {
		t.Fatalf("empresas length = %d, want 3", len(empresasRaw))
	}
	for i, raw := range empresasRaw {
		emp, _ := raw.(map[string]any)
		score, _ := emp["risk_score"].(float64)
		if score != 35 {
			t.Errorf("empresa[%d] risk_score = %v, want 35", i, score)
		}
		if emp["risk_level"] != "medio" {
			t.Errorf("empresa[%d] risk_level = %q, want medio", i, emp["risk_level"])
		}
		sanctioned, _ := emp["sanctioned"].(bool)
		if !sanctioned {
			t.Errorf("empresa[%d] sanctioned = false, want true", i)
		}
	}
}
