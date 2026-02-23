package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// newLegislativoRouter builds a minimal Chi router for the legislativo handler.
func newLegislativoRouter(h *handlers.LegislativoHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/legislativo/deputados", h.GetDeputados)
	r.Get("/v1/legislativo/deputados/{id}", h.GetDeputado)
	r.Get("/v1/legislativo/proposicoes", h.GetProposicoes)
	r.Get("/v1/legislativo/senado/materias", h.GetMateriasSenado)
	return r
}

// mockLegislativo starts a test server that returns the given body and status code.
// It returns a *handlers.LegislativoHandler whose HTTP client points at the test server.
func mockLegislativo(t *testing.T, statusCode int, body string) (*handlers.LegislativoHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	h := handlers.NewLegislativoHandlerWithClient(&http.Client{
		Transport: &redirectTransport{base: srv.URL},
	})
	return h, srv
}

func TestGetDeputados_OK(t *testing.T) {
	body := `{"dados":[{"id":1,"nome":"X"}],"links":[]}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	router := newLegislativoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/deputados", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "camara_deputados" {
		t.Errorf("Source = %q, want camara_deputados", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	total, ok := resp.Data["total"].(float64)
	if !ok || total != 1 {
		t.Errorf("Data[total] = %v, want 1", resp.Data["total"])
	}
}

func TestGetDeputados_WithFilters(t *testing.T) {
	body := `{"dados":[{"id":2,"nome":"Y"}],"links":[]}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	router := newLegislativoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/deputados?uf=SP&partido=PT", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "camara_deputados" {
		t.Errorf("Source = %q, want camara_deputados", resp.Source)
	}
}

func TestGetDeputado_OK(t *testing.T) {
	body := `{"dados":{"id":1,"nome":"X"}}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	router := newLegislativoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/deputados/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "camara_deputados" {
		t.Errorf("Source = %q, want camara_deputados", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
}

func TestGetDeputado_InvalidID(t *testing.T) {
	h := handlers.NewLegislativoHandler()
	router := newLegislativoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/deputados/abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric ID, got %d: %s", rec.Code, rec.Body.String())
	}

	var errBody map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errBody); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errBody["error"] == "" {
		t.Error("expected non-empty 'error' field in 400 response")
	}
}

func TestGetDeputado_NotFound(t *testing.T) {
	body := `{"error":"not found"}`
	h, srv := mockLegislativo(t, http.StatusNotFound, body)
	defer srv.Close()

	router := newLegislativoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/deputados/99999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetProposicoes_OK(t *testing.T) {
	body := `{"dados":[{"id":1}],"links":[]}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	router := newLegislativoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/proposicoes", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "camara_proposicoes" {
		t.Errorf("Source = %q, want camara_proposicoes", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
}

func TestGetMateriasSenado_OK(t *testing.T) {
	body := `{"Materias":{"Materia":[{"CodigoMateria":"1"}]}}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	router := newLegislativoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/senado/materias", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "senado_materias" {
		t.Errorf("Source = %q, want senado_materias", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	total, ok := resp.Data["total"].(float64)
	if !ok || total != 1 {
		t.Errorf("Data[total] = %v, want 1", resp.Data["total"])
	}
}
