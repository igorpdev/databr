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
	r.Get("/v1/legislativo/eventos", h.GetEventos)
	r.Get("/v1/legislativo/comissoes", h.GetComissoes)
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

func TestGetEventos_OK(t *testing.T) {
	body := `{"dados":[{"id":12345,"descricaoTipo":"Reunião de Comissão"}],"links":[]}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	router := newLegislativoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/eventos", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "camara_eventos" {
		t.Errorf("Source = %q, want camara_eventos", resp.Source)
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

func TestGetEventos_WithFilters(t *testing.T) {
	// The mock captures the incoming request URL so we can verify query params.
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"dados":[],"links":[]}`))
	}))
	defer srv.Close()

	h := handlers.NewLegislativoHandlerWithClient(&http.Client{
		Transport: &redirectTransport{base: srv.URL},
	})

	router := newLegislativoRouter(h)
	req := httptest.NewRequest(http.MethodGet,
		"/v1/legislativo/eventos?dataInicio=2026-02-21&dataFim=2026-02-28&orgao=PLEN", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if capturedURL == "" {
		t.Fatal("mock server was never called")
	}
	for _, want := range []string{"dataInicio=2026-02-21", "dataFim=2026-02-28", "siglaOrgao=PLEN"} {
		if !containsSubstr(capturedURL, want) {
			t.Errorf("upstream URL %q missing param %q", capturedURL, want)
		}
	}
}

func TestGetComissoes_OK(t *testing.T) {
	body := `{"dados":[{"id":1,"sigla":"CCJC","nome":"Constituição, Justiça e Cidadania"}],"links":[]}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	router := newLegislativoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/comissoes", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "camara_comissoes" {
		t.Errorf("Source = %q, want camara_comissoes", resp.Source)
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

// containsSubstr is a small helper used by filter tests.
func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstrInner(s, sub))
}

func containsSubstrInner(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestGetFrentes_OK(t *testing.T) {
	body := `{"dados":[{"id":1,"titulo":"Frente Parlamentar da Agropecuária"}],"links":[]}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/legislativo/frentes", h.GetFrentes)

	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/frentes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	data := resp["data"].(map[string]any)
	frentes, _ := data["frentes"].([]any)
	if len(frentes) == 0 {
		t.Error("expected at least one frente")
	}
}

func TestGetBlocos_OK(t *testing.T) {
	body := `{"dados":[{"id":1,"nome":"Bloco Parlamentar"}],"links":[]}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/legislativo/blocos", h.GetBlocos)

	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/blocos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetDespesas_OK(t *testing.T) {
	body := `{"dados":[{"ano":2026,"mes":1,"valorLiquido":1500.00}],"links":[]}`
	h, srv := mockLegislativo(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/legislativo/deputados/{id}/despesas", h.GetDespesas)

	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/deputados/73291/despesas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetDespesas_InvalidID(t *testing.T) {
	h := handlers.NewLegislativoHandler()
	r := chi.NewRouter()
	r.Get("/v1/legislativo/deputados/{id}/despesas", h.GetDespesas)

	req := httptest.NewRequest(http.MethodGet, "/v1/legislativo/deputados/abc/despesas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rec.Code)
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
