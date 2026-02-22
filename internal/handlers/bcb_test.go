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

type stubBCBStore struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubBCBStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *stubBCBStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	for _, r := range s.records {
		if r.Source == source && r.RecordKey == key {
			return &r, nil
		}
	}
	return nil, nil
}

func newBCBRouter(h *handlers.BCBHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/bcb/selic", h.GetSelic)
	r.Get("/v1/bcb/cambio/{moeda}", h.GetCambio)
	return r
}

func TestBCBHandler_GetSelic_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{
			{
				Source:    "bcb_selic",
				RecordKey: "20/02/2026",
				Data:      map[string]any{"data": "20/02/2026", "valor": "0.055131"},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewBCBHandler(store)
	r := newBCBRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/selic", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "bcb_selic" {
		t.Errorf("Source = %q, want bcb_selic", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
}

func TestBCBHandler_GetCambio_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{
			{
				Source:    "bcb_ptax",
				RecordKey: "USD_2026-02-20",
				Data: map[string]any{
					"moeda":          "USD",
					"cotacao_compra": 5.75,
					"cotacao_venda":  5.76,
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewBCBHandler(store)
	r := newBCBRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/cambio/USD", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestBCBHandler_GetCambio_NotFound(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewBCBHandler(store)
	r := newBCBRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/cambio/USD", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
