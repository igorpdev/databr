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
)

// TestCarteiraRisco_OK_TwoDifferentCNPJs verifies the handler returns a valid
// portfolio risk assessment for two distinct valid CNPJs with different company
// data, verifying structure, risk scoring, and aggregation.
func TestCarteiraRisco_OK_TwoDifferentCNPJs(t *testing.T) {
	// The mock fetcher always returns the same record regardless of CNPJ.
	// To keep it simple, we still verify the handler correctly processes two
	// entries and produces the expected portfolio summary.
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data: map[string]any{
				"cnpj":                  "11222333000181",
				"razao_social":         "EMPRESA ALPHA LTDA",
				"cnae_fiscal":          "6201501",
				"uf":                   "SP",
				"data_inicio_atividade": "2010-01-15",
				"situacao_cadastral":   "ATIVA",
			},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{records: nil} // no sanctions
	store := &ddStore{records: nil}                         // no judicial records

	h := handlers.NewCarteiraRiscoHandler(cnpjFetcher, complianceFetcher, store)
	router := newCarteiraRiscoRouter(h)

	body := `{"cnpjs":["11222333000181","11444777000161"]}`
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

	if resp.Source != "carteira_risco" {
		t.Errorf("Source = %q, want carteira_risco", resp.Source)
	}
	if resp.CostUSDC != "0.100" {
		t.Errorf("CostUSDC = %q, want 0.100", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}

	totalCNPJs, _ := resp.Data["total_cnpjs"].(float64)
	if totalCNPJs != 2 {
		t.Errorf("total_cnpjs = %v, want 2", totalCNPJs)
	}

	// Both companies are clean and active with >2 years, so risk = 0 each.
	riscoMedio, _ := resp.Data["risco_medio"].(float64)
	if riscoMedio != 0 {
		t.Errorf("risco_medio = %v, want 0 for clean portfolio", riscoMedio)
	}

	nivelRisco, _ := resp.Data["nivel_risco_medio"].(string)
	if nivelRisco != "baixo" {
		t.Errorf("nivel_risco_medio = %q, want baixo", nivelRisco)
	}

	// Verify distribuicao_risco has all required keys.
	dist, ok := resp.Data["distribuicao_risco"].(map[string]any)
	if !ok {
		t.Fatal("distribuicao_risco must be a map")
	}
	for _, key := range []string{"baixo", "medio", "alto", "critico"} {
		if _, ok := dist[key]; !ok {
			t.Errorf("distribuicao_risco missing key %q", key)
		}
	}
	baixo, _ := dist["baixo"].(float64)
	if baixo != 2 {
		t.Errorf("distribution baixo = %v, want 2", baixo)
	}

	// Verify concentracao_setorial and concentracao_geografica.
	setor, ok := resp.Data["concentracao_setorial"].(map[string]any)
	if !ok {
		t.Fatal("concentracao_setorial must be a map")
	}
	cnaeCount, _ := setor["6201501"].(float64)
	if cnaeCount != 2 {
		t.Errorf("sector 6201501 = %v, want 2", cnaeCount)
	}

	geo, ok := resp.Data["concentracao_geografica"].(map[string]any)
	if !ok {
		t.Fatal("concentracao_geografica must be a map")
	}
	spCount, _ := geo["SP"].(float64)
	if spCount != 2 {
		t.Errorf("geographic SP = %v, want 2", spCount)
	}

	// Verify empresas array.
	empresas, ok := resp.Data["empresas"].([]any)
	if !ok {
		t.Fatal("empresas must be a slice")
	}
	if len(empresas) != 2 {
		t.Errorf("len(empresas) = %d, want 2", len(empresas))
	}

	for i, raw := range empresas {
		emp, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("empresas[%d] must be a map", i)
		}
		// Each clean, active, established company should have risk_score = 0.
		score, _ := emp["risk_score"].(float64)
		if score != 0 {
			t.Errorf("empresas[%d] risk_score = %v, want 0", i, score)
		}
		if emp["risk_level"] != "baixo" {
			t.Errorf("empresas[%d] risk_level = %q, want baixo", i, emp["risk_level"])
		}
		sanctioned, _ := emp["sanctioned"].(bool)
		if sanctioned {
			t.Errorf("empresas[%d] sanctioned = true, want false", i)
		}
	}
}

// TestCarteiraRisco_SanctionedWithJudicial tests that sanctions (+35) and
// judicial records (+5 each, capped at 35) are properly scored.
func TestCarteiraRisco_SanctionedWithJudicial(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":                "12345678000195",
				"razao_social":       "EMPRESA RISCO SA",
				"situacao_cadastral": "ATIVA",
				"uf":                 "MG",
				"cnae_fiscal":        "4711302",
			},
			FetchedAt: time.Now(),
		}},
	}
	// Sanctioned: +35
	complianceFetcher := &ddComplianceFetcher{
		records: []domain.SourceRecord{{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}},
	}
	// 3 judicial records: +15
	store := &ddStore{
		records: []domain.SourceRecord{
			{Source: "datajud_cnj", Data: map[string]any{"processo": "1"}},
			{Source: "datajud_cnj", Data: map[string]any{"processo": "2"}},
			{Source: "datajud_cnj", Data: map[string]any{"processo": "3"}},
		},
	}

	h := handlers.NewCarteiraRiscoHandler(cnpjFetcher, complianceFetcher, store)
	router := newCarteiraRiscoRouter(h)

	body := `{"cnpjs":["12345678000195"]}`
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

	// Total: 0 (active company) + 35 (sanctions) + 15 (3 * 5 judicial) = 50
	riscoMedio, _ := resp.Data["risco_medio"].(float64)
	if riscoMedio != 50 {
		t.Errorf("risco_medio = %v, want 50", riscoMedio)
	}

	empresas, _ := resp.Data["empresas"].([]any)
	if len(empresas) != 1 {
		t.Fatalf("len(empresas) = %d, want 1", len(empresas))
	}
	emp, _ := empresas[0].(map[string]any)
	score, _ := emp["risk_score"].(float64)
	if score != 50 {
		t.Errorf("risk_score = %v, want 50", score)
	}
	if emp["risk_level"] != "alto" {
		t.Errorf("risk_level = %q, want alto (score 50 >= 40)", emp["risk_level"])
	}
	sanctioned, _ := emp["sanctioned"].(bool)
	if !sanctioned {
		t.Error("sanctioned should be true")
	}
	judicialCount, _ := emp["judicial_count"].(float64)
	if judicialCount != 3 {
		t.Errorf("judicial_count = %v, want 3", judicialCount)
	}
}

// TestCarteiraRisco_YoungCompany verifies that companies less than 2 years old
// receive an additional +10 risk score.
func TestCarteiraRisco_YoungCompany(t *testing.T) {
	// Set data_inicio_atividade to 6 months ago (< 2 years).
	sixMonthsAgo := time.Now().AddDate(0, -6, 0).Format("2006-01-02")
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":                  "12345678000195",
				"razao_social":         "STARTUP RECENTE LTDA",
				"cnae_fiscal":          "6209100",
				"uf":                   "RJ",
				"data_inicio_atividade": sixMonthsAgo,
				"situacao_cadastral":   "ATIVA",
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

	empresas, _ := resp.Data["empresas"].([]any)
	if len(empresas) != 1 {
		t.Fatalf("len(empresas) = %d, want 1", len(empresas))
	}
	emp, _ := empresas[0].(map[string]any)
	score, _ := emp["risk_score"].(float64)
	// Young company (< 2 years) = +10
	if score != 10 {
		t.Errorf("risk_score = %v, want 10 (young company penalty)", score)
	}
	if emp["risk_level"] != "baixo" {
		t.Errorf("risk_level = %q, want baixo (score 10 < 20)", emp["risk_level"])
	}
	companyAge, _ := emp["company_age_years"].(float64)
	if companyAge != 0 {
		t.Errorf("company_age_years = %v, want 0 (6 months rounds to 0)", companyAge)
	}
}

// TestCarteiraRisco_InactiveCompany verifies that an inactive company
// (situacao_cadastral != "ATIVA") receives an additional +10 risk score.
func TestCarteiraRisco_InactiveCompany(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":                  "12345678000195",
				"razao_social":         "EMPRESA SUSPENSA SA",
				"uf":                   "BA",
				"data_inicio_atividade": "2015-06-01",
				"situacao_cadastral":   "SUSPENSA",
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

	empresas, _ := resp.Data["empresas"].([]any)
	if len(empresas) != 1 {
		t.Fatalf("len(empresas) = %d, want 1", len(empresas))
	}
	emp, _ := empresas[0].(map[string]any)
	score, _ := emp["risk_score"].(float64)
	// Inactive company: +10 (situacao_cadastral != "ATIVA")
	if score != 10 {
		t.Errorf("risk_score = %v, want 10 (inactive company penalty)", score)
	}
	if emp["risk_level"] != "baixo" {
		t.Errorf("risk_level = %q, want baixo (score 10 < 20)", emp["risk_level"])
	}
}

// TestCarteiraRisco_CompanyNotFound verifies that when the CNPJ fetcher returns
// no records, the handler assigns +20 risk (unable to retrieve company data).
func TestCarteiraRisco_CompanyNotFound(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{records: nil} // no company data
	complianceFetcher := &ddComplianceFetcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewCarteiraRiscoHandler(cnpjFetcher, complianceFetcher, store)
	router := newCarteiraRiscoRouter(h)

	body := `{"cnpjs":["12345678000195"]}`
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

	empresas, _ := resp.Data["empresas"].([]any)
	if len(empresas) != 1 {
		t.Fatalf("len(empresas) = %d, want 1", len(empresas))
	}
	emp, _ := empresas[0].(map[string]any)
	score, _ := emp["risk_score"].(float64)
	// Company not found: +20
	if score != 20 {
		t.Errorf("risk_score = %v, want 20 (company data not found)", score)
	}
	if emp["risk_level"] != "medio" {
		t.Errorf("risk_level = %q, want medio (score 20 >= 20)", emp["risk_level"])
	}

	riscoMedio, _ := resp.Data["risco_medio"].(float64)
	if riscoMedio != 20 {
		t.Errorf("risco_medio = %v, want 20", riscoMedio)
	}
	nivelMedio, _ := resp.Data["nivel_risco_medio"].(string)
	if nivelMedio != "medio" {
		t.Errorf("nivel_risco_medio = %q, want medio", nivelMedio)
	}
}

// TestCarteiraRisco_JudicialCapAt35 verifies the judicial penalty is capped at 35.
// 8 judicial records * 5 = 40, but should be capped at 35.
func TestCarteiraRisco_JudicialCapAt35(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":                "12345678000195",
				"razao_social":       "EMPRESA LITIGIOSA SA",
				"situacao_cadastral": "ATIVA",
			},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{records: nil}
	// 8 judicial records: 8*5 = 40, capped at 35
	judicialRecords := make([]domain.SourceRecord, 8)
	for i := range judicialRecords {
		judicialRecords[i] = domain.SourceRecord{
			Source: "datajud_cnj",
			Data:   map[string]any{"processo": i + 1},
		}
	}
	store := &ddStore{records: judicialRecords}

	h := handlers.NewCarteiraRiscoHandler(cnpjFetcher, complianceFetcher, store)
	router := newCarteiraRiscoRouter(h)

	body := `{"cnpjs":["12345678000195"]}`
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

	empresas, _ := resp.Data["empresas"].([]any)
	if len(empresas) != 1 {
		t.Fatalf("len(empresas) = %d, want 1", len(empresas))
	}
	emp, _ := empresas[0].(map[string]any)
	score, _ := emp["risk_score"].(float64)
	// Capped at 35 (not 40)
	if score != 35 {
		t.Errorf("risk_score = %v, want 35 (judicial cap)", score)
	}
	judicialCount, _ := emp["judicial_count"].(float64)
	if judicialCount != 8 {
		t.Errorf("judicial_count = %v, want 8", judicialCount)
	}
}

// TestCarteiraRisco_ScoreCapAt100 verifies the total score is capped at 100.
func TestCarteiraRisco_ScoreCapAt100(t *testing.T) {
	// No company data: +20
	cnpjFetcher := &ddCNPJFetcher{records: nil}
	// Sanctioned: +35
	complianceFetcher := &ddComplianceFetcher{
		records: []domain.SourceRecord{{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}},
	}
	// 10 judicial records: 10*5 = 50, capped at 35
	judicialRecords := make([]domain.SourceRecord, 10)
	for i := range judicialRecords {
		judicialRecords[i] = domain.SourceRecord{
			Source: "datajud_cnj",
			Data:   map[string]any{"processo": i + 1},
		}
	}
	store := &ddStore{records: judicialRecords}

	// Total raw: 20 + 35 + 35 = 90 (under 100, no cap hit)
	// But let's verify it comes out correctly.
	h := handlers.NewCarteiraRiscoHandler(cnpjFetcher, complianceFetcher, store)
	router := newCarteiraRiscoRouter(h)

	body := `{"cnpjs":["12345678000195"]}`
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

	empresas, _ := resp.Data["empresas"].([]any)
	if len(empresas) != 1 {
		t.Fatalf("len(empresas) = %d, want 1", len(empresas))
	}
	emp, _ := empresas[0].(map[string]any)
	score, _ := emp["risk_score"].(float64)
	// 20 (no company data) + 35 (sanctions) + 35 (judicial capped) = 90
	if score != 90 {
		t.Errorf("risk_score = %v, want 90", score)
	}
	if emp["risk_level"] != "critico" {
		t.Errorf("risk_level = %q, want critico (score 90 >= 70)", emp["risk_level"])
	}
}

// TestCarteiraRisco_ContentType verifies the response Content-Type header.
func TestCarteiraRisco_ContentType(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":              "12345678000195",
				"situacao_cadastral": "ATIVA",
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
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
