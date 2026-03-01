package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

func TestBNDESHandler_GetOperacoes_OK(t *testing.T) {
	bndesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "package_show") {
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": map[string]any{
					"resources": []map[string]any{
						{"id": "fake-resource-id", "format": "CSV"},
					},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "datastore_search") {
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": map[string]any{
					"records": []map[string]any{
						{"cnpj_empresa": "12345678000195", "valor_contratado": 1000000},
					},
					"total": 1,
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer bndesSrv.Close()

	h := handlers.NewBNDESHandlerWithBaseURL(bndesSrv.URL)
	rtr := chi.NewRouter()
	rtr.Get("/v1/bndes/operacoes/{cnpj}", h.GetOperacoes)

	req := httptest.NewRequest(http.MethodGet, "/v1/bndes/operacoes/12345678000195", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	rec := httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "bndes_operacoes" {
		t.Errorf("Source = %q, want bndes_operacoes", resp.Source)
	}
	if resp.CostUSDC != "0.005" {
		t.Errorf("CostUSDC = %q, want 0.005", resp.CostUSDC)
	}
}

func TestBNDESHandler_GetOperacoes_InvalidCNPJ(t *testing.T) {
	h := handlers.NewBNDESHandlerWithBaseURL("http://unused")
	rtr := chi.NewRouter()
	rtr.Get("/v1/bndes/operacoes/{cnpj}", h.GetOperacoes)

	req := httptest.NewRequest(http.MethodGet, "/v1/bndes/operacoes/123", nil)
	rec := httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestBNDESHandler_GetOperacoes_NotFound(t *testing.T) {
	bndesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "package_show") {
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  map[string]any{"resources": []map[string]any{{"id": "res-id"}}},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"result":  map[string]any{"records": []any{}, "total": 0},
		})
	}))
	defer bndesSrv.Close()

	h := handlers.NewBNDESHandlerWithBaseURL(bndesSrv.URL)
	rtr := chi.NewRouter()
	rtr.Get("/v1/bndes/operacoes/{cnpj}", h.GetOperacoes)

	req := httptest.NewRequest(http.MethodGet, "/v1/bndes/operacoes/99999999000191", nil)
	rec := httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
