package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// csCNPJFetcher implements handlers.CNPJFetcher for credit score tests.
type csCNPJFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *csCNPJFetcher) FetchByCNPJ(ctx context.Context, cnpj string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

// csComplianceFetcher implements handlers.ComplianceFetcher for credit score tests.
type csComplianceFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *csComplianceFetcher) FetchByCNPJ(ctx context.Context, cnpj string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *csComplianceFetcher) FetchGranularByCNPJ(ctx context.Context, cnpj, list string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

// csJudicialSearcher implements handlers.DataJudSearcher for credit score tests.
type csJudicialSearcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *csJudicialSearcher) Search(ctx context.Context, doc string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

// csStore implements handlers.SourceStore for credit score tests.
type csStore struct {
	filteredRecords []domain.SourceRecord
	err             error
}

func (s *csStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	return nil, nil
}

func (s *csStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	return nil, nil
}

func (s *csStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	return s.filteredRecords, s.err
}

func newCreditoScoreRouter(h *handlers.CreditoScoreHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/credito/score/{cnpj}", h.GetCreditoScore)
	return r
}

func TestCreditoScore_BaseScore(t *testing.T) {
	// No sanctions, no judicial, no contracts, company < 5 years => score = 70
	cnpjFetcher := &csCNPJFetcher{
		records: []domain.SourceRecord{{
			Source: "cnpj",
			Data: map[string]any{
				"cnpj":                    "12345678000195",
				"data_inicio_atividade":   "2023-01-01",
			},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &csComplianceFetcher{records: nil}
	judicialSearcher := &csJudicialSearcher{records: nil}
	store := &csStore{filteredRecords: nil}

	h := handlers.NewCreditoScoreHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newCreditoScoreRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/credito/score/12345678000195", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "credito_score" {
		t.Errorf("Source = %q, want credito_score", resp.Source)
	}
	if resp.CostUSDC != "0.050" {
		t.Errorf("CostUSDC = %q, want 0.050", resp.CostUSDC)
	}
	score, _ := resp.Data["score"].(float64)
	if score != 70 {
		t.Errorf("score = %v, want 70", score)
	}
	rating, _ := resp.Data["rating"].(string)
	if rating != "B" {
		t.Errorf("rating = %q, want B", rating)
	}
}

func TestCreditoScore_MaxBonus(t *testing.T) {
	// Old company (>5y) + gov contracts = 70 + 10 + 10 = 90
	cnpjFetcher := &csCNPJFetcher{
		records: []domain.SourceRecord{{
			Source: "cnpj",
			Data: map[string]any{
				"cnpj":                    "12345678000195",
				"data_inicio_atividade":   "2015-01-01",
			},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &csComplianceFetcher{records: nil}
	judicialSearcher := &csJudicialSearcher{records: nil}
	store := &csStore{
		filteredRecords: []domain.SourceRecord{{
			Source: "pncp_licitacoes",
			Data:   map[string]any{"cnpj": "12345678000195", "contrato": "001"},
		}},
	}

	h := handlers.NewCreditoScoreHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newCreditoScoreRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/credito/score/12345678000195", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	score, _ := resp.Data["score"].(float64)
	if score != 90 {
		t.Errorf("score = %v, want 90 (70 + 10 age + 10 contracts)", score)
	}
	rating, _ := resp.Data["rating"].(string)
	if rating != "A" {
		t.Errorf("rating = %q, want A", rating)
	}
}

func TestCreditoScore_SanctionsPenalty(t *testing.T) {
	// Sanctions = 70 - 30 = 40
	cnpjFetcher := &csCNPJFetcher{
		records: []domain.SourceRecord{{
			Source: "cnpj",
			Data:   map[string]any{"cnpj": "12345678000195"},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &csComplianceFetcher{
		records: []domain.SourceRecord{{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}},
	}
	judicialSearcher := &csJudicialSearcher{records: nil}
	store := &csStore{filteredRecords: nil}

	h := handlers.NewCreditoScoreHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newCreditoScoreRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/credito/score/12345678000195", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	score, _ := resp.Data["score"].(float64)
	if score != 40 {
		t.Errorf("score = %v, want 40 (70 - 30)", score)
	}
	rating, _ := resp.Data["rating"].(string)
	if rating != "C" {
		t.Errorf("rating = %q, want C", rating)
	}
}

func TestCreditoScore_JudicialPenalty(t *testing.T) {
	// 3 judicial processes = 70 - 15 = 55
	cnpjFetcher := &csCNPJFetcher{
		records: []domain.SourceRecord{{
			Source: "cnpj",
			Data:   map[string]any{"cnpj": "12345678000195"},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &csComplianceFetcher{records: nil}
	judicialSearcher := &csJudicialSearcher{
		records: []domain.SourceRecord{
			{Source: "datajud_cnj", Data: map[string]any{"processo": "1"}},
			{Source: "datajud_cnj", Data: map[string]any{"processo": "2"}},
			{Source: "datajud_cnj", Data: map[string]any{"processo": "3"}},
		},
	}
	store := &csStore{filteredRecords: nil}

	h := handlers.NewCreditoScoreHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newCreditoScoreRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/credito/score/12345678000195", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	score, _ := resp.Data["score"].(float64)
	if score != 55 {
		t.Errorf("score = %v, want 55 (70 - 15)", score)
	}
}

func TestCreditoScore_ClampToZero(t *testing.T) {
	// Sanctions (-30) + 14 judicial processes (-70) = 70 - 30 - 70 = -30, clamped to 0
	cnpjFetcher := &csCNPJFetcher{
		records: []domain.SourceRecord{{
			Source: "cnpj",
			Data:   map[string]any{"cnpj": "12345678000195"},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &csComplianceFetcher{
		records: []domain.SourceRecord{{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}},
	}
	judicialRecords := make([]domain.SourceRecord, 14)
	for i := range judicialRecords {
		judicialRecords[i] = domain.SourceRecord{Source: "datajud_cnj", Data: map[string]any{"processo": i}}
	}
	judicialSearcher := &csJudicialSearcher{records: judicialRecords}
	store := &csStore{filteredRecords: nil}

	h := handlers.NewCreditoScoreHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newCreditoScoreRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/credito/score/12345678000195", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	score, _ := resp.Data["score"].(float64)
	if score != 0 {
		t.Errorf("score = %v, want 0 (clamped)", score)
	}
}

func TestCreditoScore_InvalidCNPJ(t *testing.T) {
	h := handlers.NewCreditoScoreHandler(
		&csCNPJFetcher{},
		&csComplianceFetcher{},
		&csJudicialSearcher{},
		&csStore{},
	)
	router := newCreditoScoreRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/credito/score/123", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
