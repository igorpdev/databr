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
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

type stubDataJudSearcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubDataJudSearcher) Search(ctx context.Context, documento string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func TestJudicialHandler_GetProcessos_OK(t *testing.T) {
	searcher := &stubDataJudSearcher{
		records: []domain.SourceRecord{{
			Source:    "datajud_cnj",
			RecordKey: "0001234-56.2023.8.26.0001",
			Data:      map[string]any{"numeroProcesso": "0001234-56.2023.8.26.0001", "tribunal": "TJSP"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewJudicialHandler(searcher)
	r := chi.NewRouter()
	r.Get("/v1/judicial/processos/{doc}", h.GetProcessos)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processos/12345678909", nil)
	req = x402pkg.InjectPrice(req, "0.015")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CostUSDC != "0.015" {
		t.Errorf("CostUSDC = %q, want 0.015", resp.CostUSDC)
	}
}

func TestJudicialHandler_GetProcessos_NotFound(t *testing.T) {
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	r := chi.NewRouter()
	r.Get("/v1/judicial/processos/{doc}", h.GetProcessos)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processos/99999999999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
