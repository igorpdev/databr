package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func newEconomiaRouter(h *handlers.EconomiaHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/economia/ipca", h.GetIPCA)
	r.Get("/v1/economia/pib", h.GetPIB)
	return r
}

func TestEconomiaHandler_GetIPCA_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{
			{
				Source:    "ibge_ipca",
				RecordKey: "202601",
				Data: map[string]any{
					"periodo":      "202601",
					"variacao_pct": "0.16",
					"indicador":    "IPCA",
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewEconomiaHandler(store)
	r := newEconomiaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/economia/ipca", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Source != "ibge_ipca" {
		t.Errorf("Source = %q, want ibge_ipca", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
}

func TestEconomiaHandler_GetPIB_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{
			{
				Source:    "ibge_pib",
				RecordKey: "202503",
				Data:      map[string]any{"periodo": "202503", "valor": "2800000", "indicador": "PIB"},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewEconomiaHandler(store)
	r := newEconomiaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/economia/pib", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestEconomiaHandler_GetIPCA_NoData(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewEconomiaHandler(store)
	r := newEconomiaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/economia/ipca", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no data, got %d", rec.Code)
	}
}
