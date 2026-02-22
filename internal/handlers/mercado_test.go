package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func newMercadoRouter(h *handlers.MercadoHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/mercado/acoes/{ticker}", h.GetAcoes)
	r.Get("/v1/mercado/fundos/{cnpj}", h.GetFundos)
	return r
}

func TestMercadoHandler_GetAcoes_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "b3_cotacoes",
			RecordKey: "PETR4",
			Data:      map[string]any{"ticker": "PETR4", "preco_fechamento": 35.50},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/acoes/PETR4", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMercadoHandler_GetAcoes_NotFound(t *testing.T) {
	store := &stubBCBStore{}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/acoes/XXXX3", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestMercadoHandler_GetFundos_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "cvm_fundos",
			RecordKey: "12345678000195",
			Data:      map[string]any{"cnpj_fundo": "12345678000195", "denom_social": "FUNDO XYZ FIA"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMercadoHandler_GetFundos_NotFound(t *testing.T) {
	store := &stubBCBStore{}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/99999999000199", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
