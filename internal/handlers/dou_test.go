package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/collectors/dou"
	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

type stubQDFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubQDFetcher) Search(ctx context.Context, params dou.SearchParams) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func TestDOUHandler_GetBusca_OK(t *testing.T) {
	fetcher := &stubQDFetcher{
		records: []domain.SourceRecord{{
			Source:    "querido_diario",
			RecordKey: "contrato_0",
			Data:      map[string]any{"territory_name": "São Paulo", "date": "2026-02-01"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewDOUHandler(fetcher)
	r := chi.NewRouter()
	r.Get("/v1/dou/busca", h.GetBusca)

	req := httptest.NewRequest(http.MethodGet, "/v1/dou/busca?q=contrato", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", resp.Source)
	}
}

func TestDOUHandler_GetBusca_MissingQuery(t *testing.T) {
	h := handlers.NewDOUHandler(&stubQDFetcher{})
	r := chi.NewRouter()
	r.Get("/v1/dou/busca", h.GetBusca)

	req := httptest.NewRequest(http.MethodGet, "/v1/dou/busca", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
