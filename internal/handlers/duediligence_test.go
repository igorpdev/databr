package handlers_test

import (
	"context"
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

// ddCNPJFetcher implements handlers.CNPJFetcher for due diligence tests.
type ddCNPJFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *ddCNPJFetcher) FetchByCNPJ(ctx context.Context, cnpj string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

// ddComplianceFetcher implements handlers.ComplianceFetcher for due diligence tests.
type ddComplianceFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *ddComplianceFetcher) FetchByCNPJ(ctx context.Context, cnpj string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *ddComplianceFetcher) FetchGranularByCNPJ(ctx context.Context, cnpj, list string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

// ddJudicialSearcher implements handlers.DataJudSearcher for due diligence tests.
type ddJudicialSearcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *ddJudicialSearcher) Search(ctx context.Context, doc string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

// ddStore implements handlers.SourceStore for due diligence tests.
type ddStore struct {
	records []domain.SourceRecord
	err     error
}

func (s *ddStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *ddStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	for _, r := range s.records {
		if r.Source == source && r.RecordKey == key {
			return &r, nil
		}
	}
	return nil, s.err
}

func (s *ddStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func newDueDiligenceRouter(h *handlers.DueDiligenceHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/empresas/{cnpj}/duediligence", h.GetDueDiligence)
	return r
}

func TestDueDiligence_OK_LowRisk(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data:      map[string]any{"cnpj": "12345678000195", "razao_social": "EMPRESA XPTO LTDA"},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{records: nil}
	judicialSearcher := &ddJudicialSearcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewDueDiligenceHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newDueDiligenceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/duediligence", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "duediligence" {
		t.Errorf("Source = %q, want duediligence", resp.Source)
	}
	if resp.CostUSDC != "0.050" {
		t.Errorf("CostUSDC = %q, want 0.050", resp.CostUSDC)
	}
	riskScore, _ := resp.Data["risk_score"].(float64)
	if riskScore != 0 {
		t.Errorf("risk_score = %v, want 0 for clean company", riskScore)
	}
	riskLevel, _ := resp.Data["risk_level"].(string)
	if riskLevel != "low" {
		t.Errorf("risk_level = %q, want low", riskLevel)
	}
}

func TestDueDiligence_HighRisk(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{records: nil} // no company data = +20
	complianceFetcher := &ddComplianceFetcher{
		records: []domain.SourceRecord{{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}},
	} // sanctions = +30
	judicialSearcher := &ddJudicialSearcher{
		records: []domain.SourceRecord{
			{Source: "datajud_cnj", Data: map[string]any{"processo": "1"}},
			{Source: "datajud_cnj", Data: map[string]any{"processo": "2"}},
			{Source: "datajud_cnj", Data: map[string]any{"processo": "3"}},
		},
	} // 3 processes = +15
	store := &ddStore{records: nil}

	h := handlers.NewDueDiligenceHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newDueDiligenceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/duediligence", nil)
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
	// 20 (no company) + 30 (sanctions) + 15 (3 processes) = 65
	if riskScore != 65 {
		t.Errorf("risk_score = %v, want 65", riskScore)
	}
	riskLevel, _ := resp.Data["risk_level"].(string)
	if riskLevel != "high" {
		t.Errorf("risk_level = %q, want high", riskLevel)
	}
}

func TestDueDiligence_MediumRisk(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source: "cnpj", Data: map[string]any{"cnpj": "12345678000195"},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{
		records: []domain.SourceRecord{{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}},
	} // +30
	judicialSearcher := &ddJudicialSearcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewDueDiligenceHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newDueDiligenceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/duediligence", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	riskScore, _ := resp.Data["risk_score"].(float64)
	if riskScore != 30 {
		t.Errorf("risk_score = %v, want 30", riskScore)
	}
	riskLevel, _ := resp.Data["risk_level"].(string)
	if riskLevel != "medium" {
		t.Errorf("risk_level = %q, want medium", riskLevel)
	}
}

func TestDueDiligence_JudicialCap50(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source: "cnpj", Data: map[string]any{"cnpj": "12345678000195"},
			FetchedAt: time.Now(),
		}},
	}
	complianceFetcher := &ddComplianceFetcher{records: nil}
	// 12 processes = 60 but capped at 50
	judicialRecords := make([]domain.SourceRecord, 12)
	for i := range judicialRecords {
		judicialRecords[i] = domain.SourceRecord{Source: "datajud_cnj", Data: map[string]any{"processo": i}}
	}
	judicialSearcher := &ddJudicialSearcher{records: judicialRecords}
	store := &ddStore{records: nil}

	h := handlers.NewDueDiligenceHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newDueDiligenceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/duediligence", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	riskScore, _ := resp.Data["risk_score"].(float64)
	if riskScore != 50 {
		t.Errorf("risk_score = %v, want 50 (capped)", riskScore)
	}
}

func TestDueDiligence_InvalidCNPJ(t *testing.T) {
	h := handlers.NewDueDiligenceHandler(
		&ddCNPJFetcher{},
		&ddComplianceFetcher{},
		&ddJudicialSearcher{},
		&ddStore{},
	)
	router := newDueDiligenceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/123/duediligence", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDueDiligence_PartialErrors(t *testing.T) {
	// Even if some fetchers error, handler should still return a response.
	cnpjFetcher := &ddCNPJFetcher{err: errors.New("upstream error")}
	complianceFetcher := &ddComplianceFetcher{err: errors.New("compliance error")}
	judicialSearcher := &ddJudicialSearcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewDueDiligenceHandler(cnpjFetcher, complianceFetcher, judicialSearcher, store)
	router := newDueDiligenceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/duediligence", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even with partial errors, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// No company data (error) = +20
	riskScore, _ := resp.Data["risk_score"].(float64)
	if riskScore != 20 {
		t.Errorf("risk_score = %v, want 20", riskScore)
	}
}
