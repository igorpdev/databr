package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// ceComplianceFetcher implements handlers.ComplianceFetcher for electoral compliance tests.
type ceComplianceFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *ceComplianceFetcher) FetchByCNPJ(ctx context.Context, cnpj string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *ceComplianceFetcher) FetchGranularByCNPJ(ctx context.Context, cnpj, list string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

// ceJudicialSearcher implements handlers.DataJudSearcher for electoral compliance tests.
type ceJudicialSearcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *ceJudicialSearcher) Search(ctx context.Context, doc string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *ceJudicialSearcher) SearchByNumber(ctx context.Context, numero string) ([]domain.SourceRecord, error) {
	return nil, nil
}

// ceStore implements handlers.SourceStore for electoral compliance tests.
type ceStore struct {
	filteredRecords map[string][]domain.SourceRecord
	err             error
}

func (s *ceStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	return nil, nil
}

func (s *ceStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	return nil, nil
}

func (s *ceStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	recs := s.filteredRecords[source]
	var out []domain.SourceRecord
	needle := strings.ToUpper(jsonbValue)
	for _, r := range recs {
		v, _ := r.Data[jsonbKey].(string)
		if strings.Contains(strings.ToUpper(v), needle) {
			out = append(out, r)
		}
	}
	return out, nil
}

func newComplianceEleitoralRouter(h *handlers.ComplianceEleitoralHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/eleicoes/compliance/{cpf_cnpj}", h.GetComplianceEleitoral)
	return r
}

func TestComplianceEleitoral_Clean(t *testing.T) {
	complianceFetcher := &ceComplianceFetcher{records: nil}
	judicialSearcher := &ceJudicialSearcher{records: nil}
	store := &ceStore{filteredRecords: map[string][]domain.SourceRecord{
		"tse_candidatos": {{
			Source: "tse_candidatos",
			Data:   map[string]any{"cpf_cnpj": "12345678909", "nome": "CANDIDATO TESTE"},
		}},
	}}

	h := handlers.NewComplianceEleitoralHandler(complianceFetcher, judicialSearcher, store)
	router := newComplianceEleitoralRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/compliance/12345678909", nil)
	req = x402pkg.InjectPrice(req, "0.007")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "compliance_eleitoral" {
		t.Errorf("Source = %q, want compliance_eleitoral", resp.Source)
	}
	if resp.CostUSDC != "0.007" {
		t.Errorf("CostUSDC = %q, want 0.007", resp.CostUSDC)
	}
	status, _ := resp.Data["status"].(string)
	if status != "apto" {
		t.Errorf("status = %q, want apto", status)
	}
	sanctionsFound, _ := resp.Data["sanctions_found"].(bool)
	if sanctionsFound {
		t.Error("expected sanctions_found = false")
	}
}

func TestComplianceEleitoral_WithSanctions(t *testing.T) {
	complianceFetcher := &ceComplianceFetcher{
		records: []domain.SourceRecord{{
			Source: "cgu_compliance",
			Data:   map[string]any{"sanction": "CEIS"},
		}},
	}
	judicialSearcher := &ceJudicialSearcher{records: nil}
	store := &ceStore{filteredRecords: map[string][]domain.SourceRecord{
		"tse_candidatos": {},
	}}

	h := handlers.NewComplianceEleitoralHandler(complianceFetcher, judicialSearcher, store)
	router := newComplianceEleitoralRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/compliance/12345678909", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	status, _ := resp.Data["status"].(string)
	if status != "requer_analise" {
		t.Errorf("status = %q, want requer_analise", status)
	}
	sanctionsFound, _ := resp.Data["sanctions_found"].(bool)
	if !sanctionsFound {
		t.Error("expected sanctions_found = true")
	}
}

func TestComplianceEleitoral_WithJudicial(t *testing.T) {
	complianceFetcher := &ceComplianceFetcher{records: nil}
	judicialSearcher := &ceJudicialSearcher{
		records: []domain.SourceRecord{{
			Source: "datajud_cnj",
			Data:   map[string]any{"processo": "001"},
		}},
	}
	store := &ceStore{filteredRecords: map[string][]domain.SourceRecord{
		"tse_candidatos": {},
	}}

	h := handlers.NewComplianceEleitoralHandler(complianceFetcher, judicialSearcher, store)
	router := newComplianceEleitoralRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/compliance/12345678909", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	status, _ := resp.Data["status"].(string)
	if status != "requer_analise" {
		t.Errorf("status = %q, want requer_analise", status)
	}
	judicialCount, _ := resp.Data["judicial_count"].(float64)
	if judicialCount != 1 {
		t.Errorf("judicial_count = %v, want 1", judicialCount)
	}
}

func TestComplianceEleitoral_InvalidDoc(t *testing.T) {
	h := handlers.NewComplianceEleitoralHandler(
		&ceComplianceFetcher{},
		&ceJudicialSearcher{},
		&ceStore{filteredRecords: map[string][]domain.SourceRecord{}},
	)
	router := newComplianceEleitoralRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/compliance/123", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestComplianceEleitoral_CNPJ14Digits(t *testing.T) {
	complianceFetcher := &ceComplianceFetcher{records: nil}
	judicialSearcher := &ceJudicialSearcher{records: nil}
	store := &ceStore{filteredRecords: map[string][]domain.SourceRecord{
		"tse_candidatos": {},
	}}

	h := handlers.NewComplianceEleitoralHandler(complianceFetcher, judicialSearcher, store)
	router := newComplianceEleitoralRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/compliance/12345678000195", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for 14-digit CNPJ, got %d: %s", rec.Code, rec.Body.String())
	}
}
