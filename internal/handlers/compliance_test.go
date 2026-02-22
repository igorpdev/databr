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
	records []domain.SourceRecord
	err     error
}

func (s *stubComplianceFetcher) FetchByCNPJ(ctx context.Context, cnpjNum string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func newComplianceRouter(h *handlers.ComplianceHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/compliance/{cnpj}", h.GetCompliance)
	r.Get("/v1/empresas/{cnpj}/compliance", h.GetCompliance)
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
