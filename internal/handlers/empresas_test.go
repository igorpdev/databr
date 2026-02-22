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

// stubCNPJFetcher implements handlers.CNPJFetcher using a fixed record.
type stubCNPJFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubCNPJFetcher) FetchByCNPJ(ctx context.Context, cnpj string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func newRouter(h *handlers.EmpresasHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/empresas/{cnpj}", h.GetEmpresa)
	return r
}

func TestEmpresasHandler_GetEmpresa_OK(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{
			{
				Source:    "cnpj",
				RecordKey: "12345678000195",
				Data: map[string]any{
					"cnpj":               "12345678000195",
					"razao_social":       "EMPRESA XPTO LTDA",
					"situacao_cadastral": "ATIVA",
					"uf":                 "SP",
					"municipio":          "SAO PAULO",
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Source != "cnpj" {
		t.Errorf("Source = %q, want %q", resp.Source, "cnpj")
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Error("Data must not be nil")
	}
	if resp.Data["cnpj"] != "12345678000195" {
		t.Errorf("Data[cnpj] = %v, want 12345678000195", resp.Data["cnpj"])
	}
}

func TestEmpresasHandler_GetEmpresa_NormalizeCNPJ(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{
			{
				Source:    "cnpj",
				RecordKey: "12345678000195",
				Data:      map[string]any{"cnpj": "12345678000195"},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	// CNPJ with dots and dashes (no slash — slash is a path separator and cannot
	// appear in a URL path segment, so real clients send digits-only or dot-dash format).
	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12.345.678-0001.95", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestEmpresasHandler_GetEmpresa_InvalidCNPJ(t *testing.T) {
	h := handlers.NewEmpresasHandler(&stubCNPJFetcher{})
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid CNPJ, got %d", rec.Code)
	}
}

func TestEmpresasHandler_GetEmpresa_ContentType(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{
			{Source: "cnpj", RecordKey: "12345678000195", Data: map[string]any{}, FetchedAt: time.Now()},
		},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
