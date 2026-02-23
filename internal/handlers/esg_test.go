package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func newESGRouter(h *handlers.ESGHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/ambiental/empresa/{cnpj}/esg", h.GetESG)
	return r
}

func TestESG_OK_CleanCompany(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data: map[string]any{
				"cnpj":                "11222333000181",
				"razao_social":        "EMPRESA VERDE LTDA",
				"municipio":           "Sao Paulo",
				"uf":                  "SP",
				"situacao_cadastral":  "ATIVA",
				"qsa": []any{
					map[string]any{"nome_socio": "SOCIO A"},
					map[string]any{"nome_socio": "SOCIO B"},
				},
			},
			FetchedAt: time.Now(),
		}},
	}
	// No compliance sanctions.
	complianceFetcher := &ddComplianceFetcher{records: nil}
	// Store returns no IBAMA embargos, no DETER alerts, no PNCP contracts.
	store := &ddStore{records: nil}

	h := handlers.NewESGHandler(cnpjFetcher, complianceFetcher, store)
	router := newESGRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/empresa/11222333000181/esg", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "esg_report" {
		t.Errorf("Source = %q, want esg_report", resp.Source)
	}
	if resp.CostUSDC != "0.020" {
		t.Errorf("CostUSDC = %q, want 0.020", resp.CostUSDC)
	}

	// Clean company: E=100, S=100, G=100 => ESG = 100*0.4 + 100*0.3 + 100*0.3 = 100.
	esgScore, _ := resp.Data["esg_score"].(float64)
	if esgScore != 100 {
		t.Errorf("esg_score = %v, want 100", esgScore)
	}
	classificacao, _ := resp.Data["classificacao"].(string)
	if classificacao != "A" {
		t.Errorf("classificacao = %q, want A", classificacao)
	}

	// Check environmental score.
	ambiental, ok := resp.Data["ambiental"].(map[string]any)
	if !ok {
		t.Fatalf("ambiental not found")
	}
	envScore, _ := ambiental["score"].(float64)
	if envScore != 100 {
		t.Errorf("ambiental.score = %v, want 100", envScore)
	}

	// Check social score.
	social, ok := resp.Data["social"].(map[string]any)
	if !ok {
		t.Fatalf("social not found")
	}
	socialScore, _ := social["score"].(float64)
	if socialScore != 100 {
		t.Errorf("social.score = %v, want 100", socialScore)
	}

	// Check governance score.
	governanca, ok := resp.Data["governanca"].(map[string]any)
	if !ok {
		t.Fatalf("governanca not found")
	}
	govScore, _ := governanca["score"].(float64)
	if govScore != 100 {
		t.Errorf("governanca.score = %v, want 100", govScore)
	}
}

func TestESG_WithEmbargos(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data: map[string]any{
				"cnpj":                "11222333000181",
				"razao_social":        "EMPRESA MADEIREIRA LTDA",
				"municipio":           "Manaus",
				"uf":                  "AM",
				"situacao_cadastral":  "ATIVA",
				"qsa": []any{
					map[string]any{"nome_socio": "SOCIO A"},
					map[string]any{"nome_socio": "SOCIO B"},
				},
			},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{records: nil}
	// Store returns 2 IBAMA embargos.
	store := &ddStore{
		records: []domain.SourceRecord{
			{Source: "ibama_embargos", Data: map[string]any{"auto_infracao": "001", "cpf_cnpj": "11222333000181"}},
			{Source: "ibama_embargos", Data: map[string]any{"auto_infracao": "002", "cpf_cnpj": "11222333000181"}},
		},
	}

	h := handlers.NewESGHandler(cnpjFetcher, complianceFetcher, store)
	router := newESGRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/empresa/11222333000181/esg", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// 2 IBAMA embargos: penalty = 2*25 = 50.
	// The mock store also returns the same 2 records for DETER query: penalty = 2*5 = 10.
	// E = 100 - 50 - 10 = 40.
	ambiental, ok := resp.Data["ambiental"].(map[string]any)
	if !ok {
		t.Fatalf("ambiental not found")
	}
	envScore, _ := ambiental["score"].(float64)
	if envScore != 40 {
		t.Errorf("ambiental.score = %v, want 40", envScore)
	}

	// ESG = 40*0.4 + 100*0.3 + 100*0.3 = 16 + 30 + 30 = 76 => class B.
	esgScore, _ := resp.Data["esg_score"].(float64)
	if esgScore != 76 {
		t.Errorf("esg_score = %v, want 76", esgScore)
	}
	classificacao, _ := resp.Data["classificacao"].(string)
	if classificacao != "B" {
		t.Errorf("classificacao = %q, want B", classificacao)
	}
}

func TestESG_WithSanctions(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data: map[string]any{
				"cnpj":                "11222333000181",
				"razao_social":        "EMPRESA SANCAO LTDA",
				"municipio":           "Brasilia",
				"uf":                  "DF",
				"situacao_cadastral":  "ATIVA",
				"qsa": []any{
					map[string]any{"nome_socio": "SOCIO A"},
					map[string]any{"nome_socio": "SOCIO B"},
				},
			},
			FetchedAt: time.Now(),
		}},
	}
	// 2 compliance sanctions.
	complianceFetcher := &ddComplianceFetcher{
		records: []domain.SourceRecord{
			{Source: "cgu_compliance", Data: map[string]any{"sanction": "CEIS", "tipo": "Impedimento"}},
			{Source: "cgu_compliance", Data: map[string]any{"sanction": "CNEP", "tipo": "Suspensao"}},
		},
	}
	store := &ddStore{records: nil}

	h := handlers.NewESGHandler(cnpjFetcher, complianceFetcher, store)
	router := newESGRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/empresa/11222333000181/esg", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// 2 sanctions: penalty = 2*20 = 40 => S=100-40=60.
	social, ok := resp.Data["social"].(map[string]any)
	if !ok {
		t.Fatalf("social not found")
	}
	socialScore, _ := social["score"].(float64)
	if socialScore != 60 {
		t.Errorf("social.score = %v, want 60", socialScore)
	}

	// ESG = 100*0.4 + 60*0.3 + 100*0.3 = 40 + 18 + 30 = 88 => class A.
	esgScore, _ := resp.Data["esg_score"].(float64)
	if esgScore != 88 {
		t.Errorf("esg_score = %v, want 88", esgScore)
	}
}

func TestESG_InvalidCNPJ(t *testing.T) {
	h := handlers.NewESGHandler(
		&ddCNPJFetcher{},
		&ddComplianceFetcher{},
		&ddStore{},
	)
	router := newESGRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/empresa/123/esg", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestESG_CompanyNotFound(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{records: nil, err: nil}
	complianceFetcher := &ddComplianceFetcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewESGHandler(cnpjFetcher, complianceFetcher, store)
	router := newESGRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/empresa/11222333000181/esg", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestESG_FullPenalty(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data: map[string]any{
				"cnpj":                "11222333000181",
				"razao_social":        "EMPRESA PENALIZADA LTDA",
				"municipio":           "Porto Velho",
				"uf":                  "RO",
				"situacao_cadastral":  "INAPTA", // not ATIVA => -20 governance
				// no QSA => -10 governance
			},
			FetchedAt: time.Now(),
		}},
	}
	// 5 compliance sanctions => penalty = min(5*20, 80) = 80 => S=100-80=20.
	sanctions := make([]domain.SourceRecord, 5)
	for i := range sanctions {
		sanctions[i] = domain.SourceRecord{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}
	}
	complianceFetcher := &ddComplianceFetcher{records: sanctions}

	// 3 IBAMA embargos: penalty = min(3*25, 60) = 60 => E=100-60=40.
	embargos := make([]domain.SourceRecord, 3)
	for i := range embargos {
		embargos[i] = domain.SourceRecord{
			Source: "ibama_embargos",
			Data:   map[string]any{"auto_infracao": "EMB" + string(rune('0'+i))},
		}
	}
	store := &ddStore{records: embargos}

	h := handlers.NewESGHandler(cnpjFetcher, complianceFetcher, store)
	router := newESGRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/empresa/11222333000181/esg", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// E: 3 embargos = penalty min(75,60)=60. Mock store also returns 3 for DETER: penalty min(15,30)=15.
	// E = 100 - 60 - 15 = 25.
	ambiental, ok := resp.Data["ambiental"].(map[string]any)
	if !ok {
		t.Fatalf("ambiental not found")
	}
	envScore, _ := ambiental["score"].(float64)
	if envScore != 25 {
		t.Errorf("ambiental.score = %v, want 25", envScore)
	}

	// S: 5 sanctions = 100 penalty capped at 80 => 100-80=20.
	social, ok := resp.Data["social"].(map[string]any)
	if !ok {
		t.Fatalf("social not found")
	}
	socialScore, _ := social["score"].(float64)
	if socialScore != 20 {
		t.Errorf("social.score = %v, want 20", socialScore)
	}

	// G: no QSA (-10) + INAPTA (-20) + mock store returns 3 records for PNCP (no extra penalty).
	// G = 100 - 10 - 20 = 70.
	governanca, ok := resp.Data["governanca"].(map[string]any)
	if !ok {
		t.Fatalf("governanca not found")
	}
	govScore, _ := governanca["score"].(float64)
	if govScore != 70 {
		t.Errorf("governanca.score = %v, want 70", govScore)
	}

	// ESG = 25*0.4 + 20*0.3 + 70*0.3 = 10 + 6 + 21 = 37.
	esgScore, _ := resp.Data["esg_score"].(float64)
	if esgScore != 37 {
		t.Errorf("esg_score = %v, want 37", esgScore)
	}

	// Classification: 0-39 => D.
	classificacao, _ := resp.Data["classificacao"].(string)
	if classificacao != "D" {
		t.Errorf("classificacao = %q, want D", classificacao)
	}
}

func TestESG_FetchError(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{err: errors.New("upstream error")}
	complianceFetcher := &ddComplianceFetcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewESGHandler(cnpjFetcher, complianceFetcher, store)
	router := newESGRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/empresa/11222333000181/esg", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}
