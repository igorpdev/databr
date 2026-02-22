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

type stubSICONFIFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubSICONFIFetcher) FetchRREO(ctx context.Context, uf string, ano, periodo int) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func TestTesouroHandler_GetRREO_OK(t *testing.T) {
	fetcher := &stubSICONFIFetcher{
		records: []domain.SourceRecord{{
			Source:    "tesouro_siconfi",
			RecordKey: "SP_2024_1",
			Data:      map[string]any{"ente": "São Paulo", "uf": "SP", "an_exercicio": 2024},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewTesouroHandler(fetcher)
	r := chi.NewRouter()
	r.Get("/v1/tesouro/rreo", h.GetRREO)

	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/rreo?uf=SP&ano=2024&periodo=1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "tesouro_siconfi" {
		t.Errorf("Source = %q, want tesouro_siconfi", resp.Source)
	}
}

func TestTesouroHandler_GetRREO_MissingUF(t *testing.T) {
	h := handlers.NewTesouroHandler(&stubSICONFIFetcher{})
	r := chi.NewRouter()
	r.Get("/v1/tesouro/rreo", h.GetRREO)

	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/rreo", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
