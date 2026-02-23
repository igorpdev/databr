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

type stubComplianceFetcher struct {
	records         []domain.SourceRecord
	granularRecords []domain.SourceRecord
	err             error
}

func (s *stubComplianceFetcher) FetchByCNPJ(ctx context.Context, cnpjNum string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *stubComplianceFetcher) FetchGranularByCNPJ(ctx context.Context, cnpjNum, list string) ([]domain.SourceRecord, error) {
	if s.granularRecords != nil {
		return s.granularRecords, s.err
	}
	return s.records, s.err
}

func newComplianceRouter(h *handlers.ComplianceHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/compliance/{cnpj}", h.GetCompliance)
	r.Get("/v1/empresas/{cnpj}/compliance", h.GetCompliance)
	r.Get("/v1/compliance/ceis/{cnpj}", h.GetCEIS)
	r.Get("/v1/compliance/cnep/{cnpj}", h.GetCNEP)
	r.Get("/v1/compliance/cepim/{cnpj}", h.GetCEPIM)
	return r
}

func TestComplianceHandler_GetCompliance_OK(t *testing.T) {
	fetcher := &stubComplianceFetcher{
		records: []domain.SourceRecord{{
			Source:    "cgu_compliance",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":      "12345678000195",
				"ceis":      []any{},
				"cnep":      []any{},
				"sanitized": true,
			},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewComplianceHandler(fetcher)
	r := newComplianceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/compliance/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cgu_compliance" {
		t.Errorf("Source = %q, want cgu_compliance", resp.Source)
	}
	if resp.CostUSDC != "0.005" {
		t.Errorf("CostUSDC = %q, want 0.005", resp.CostUSDC)
	}
}

func TestComplianceHandler_GetCompliance_EmpresaRoute(t *testing.T) {
	fetcher := &stubComplianceFetcher{
		records: []domain.SourceRecord{{
			Source:    "cgu_compliance",
			RecordKey: "12345678000195",
			Data:      map[string]any{"cnpj": "12345678000195", "ceis": []any{}},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewComplianceHandler(fetcher)
	r := newComplianceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/compliance", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on empresa route, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestComplianceHandler_GetCompliance_InvalidCNPJ(t *testing.T) {
	h := handlers.NewComplianceHandler(&stubComplianceFetcher{})
	r := newComplianceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/compliance/invalid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func makeGranularRecord(source, cnpj, list string) domain.SourceRecord {
	return domain.SourceRecord{
		Source:    source,
		RecordKey: cnpj,
		Data: map[string]any{
			"cnpj":  cnpj,
			"list":  list,
			"items": []any{},
			"total": 0,
		},
		FetchedAt: time.Now(),
	}
}

func TestComplianceHandler_GetCEIS_OK(t *testing.T) {
	fetcher := &stubComplianceFetcher{
		granularRecords: []domain.SourceRecord{
			makeGranularRecord("cgu_ceis", "12345678000195", "ceis"),
		},
	}
	h := handlers.NewComplianceHandler(fetcher)
	r := newComplianceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/compliance/ceis/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cgu_ceis" {
		t.Errorf("Source = %q, want cgu_ceis", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
}

func TestComplianceHandler_GetCNEP_OK(t *testing.T) {
	fetcher := &stubComplianceFetcher{
		granularRecords: []domain.SourceRecord{
			makeGranularRecord("cgu_cnep", "12345678000195", "cnep"),
		},
	}
	h := handlers.NewComplianceHandler(fetcher)
	r := newComplianceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/compliance/cnep/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cgu_cnep" {
		t.Errorf("Source = %q, want cgu_cnep", resp.Source)
	}
}

func TestComplianceHandler_GetCEPIM_OK(t *testing.T) {
	fetcher := &stubComplianceFetcher{
		granularRecords: []domain.SourceRecord{
			makeGranularRecord("cgu_cepim", "12345678000195", "cepim"),
		},
	}
	h := handlers.NewComplianceHandler(fetcher)
	r := newComplianceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/compliance/cepim/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cgu_cepim" {
		t.Errorf("Source = %q, want cgu_cepim", resp.Source)
	}
}

func TestComplianceHandler_GetCEIS_InvalidCNPJ(t *testing.T) {
	h := handlers.NewComplianceHandler(&stubComplianceFetcher{})
	r := newComplianceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/compliance/ceis/invalid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestComplianceHandler_GetCEIS_NotFound(t *testing.T) {
	fetcher := &stubComplianceFetcher{
		granularRecords: []domain.SourceRecord{},
	}
	h := handlers.NewComplianceHandler(fetcher)
	r := newComplianceRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/compliance/ceis/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
