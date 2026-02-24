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
	records       []domain.SourceRecord
	err           error
	numberRecords []domain.SourceRecord
	numberErr     error
}

func (s *stubDataJudSearcher) Search(ctx context.Context, documento string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *stubDataJudSearcher) SearchByNumber(ctx context.Context, numero string) ([]domain.SourceRecord, error) {
	return s.numberRecords, s.numberErr
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

func TestJudicialHandler_GetProcesso_OK(t *testing.T) {
	searcher := &stubDataJudSearcher{
		numberRecords: []domain.SourceRecord{{
			Source:    "datajud_cnj",
			RecordKey: "00008323520184013202",
			Data: map[string]any{
				"numeroProcesso": "00008323520184013202",
				"tribunal":       "TRF1",
				"classe":         map[string]any{"nome": "Recurso Inominado Cível"},
			},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewJudicialHandler(searcher)
	r := chi.NewRouter()
	r.Get("/v1/judicial/processo/{numero}", h.GetProcesso)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processo/0000832-35.2018.4.01.3202", nil)
	req = x402pkg.InjectPrice(req, "0.010")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "datajud_cnj" {
		t.Errorf("Source = %q, want datajud_cnj", resp.Source)
	}
	if resp.CostUSDC != "0.010" {
		t.Errorf("CostUSDC = %q, want 0.010", resp.CostUSDC)
	}
}

func TestJudicialHandler_GetProcesso_InvalidFormat(t *testing.T) {
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	r := chi.NewRouter()
	r.Get("/v1/judicial/processo/{numero}", h.GetProcesso)

	// No hyphens or dots — invalid CNJ format
	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processo/12345678901234567890", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestJudicialHandler_GetProcesso_NotFound(t *testing.T) {
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	r := chi.NewRouter()
	r.Get("/v1/judicial/processo/{numero}", h.GetProcesso)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processo/0000832-35.2018.4.01.3202", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
