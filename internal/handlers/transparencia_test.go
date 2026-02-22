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

func newTransparenciaRouter(h *handlers.TransparenciaHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/transparencia/licitacoes", h.GetLicitacoes)
	r.Get("/v1/eleicoes/candidatos", h.GetCandidatos)
	return r
}

func TestTransparenciaHandler_GetLicitacoes_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "pncp_licitacoes",
			RecordKey: "2026000001",
			Data:      map[string]any{"numero_controle": "2026000001", "objeto": "Material de escritório"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewTransparenciaHandler(store)
	r := newTransparenciaRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/licitacoes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "pncp_licitacoes" {
		t.Errorf("Source = %q, want pncp_licitacoes", resp.Source)
	}
}

func TestTransparenciaHandler_GetCandidatos_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "tse_candidatos",
			RecordKey: "123456",
			Data:      map[string]any{"sq_candidato": "123456", "nm_candidato": "JOAO DA SILVA", "sg_uf": "SP"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewTransparenciaHandler(store)
	r := newTransparenciaRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/candidatos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTransparenciaHandler_GetLicitacoes_NoData(t *testing.T) {
	store := &stubBCBStore{}
	h := handlers.NewTransparenciaHandler(store)
	r := newTransparenciaRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/licitacoes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
