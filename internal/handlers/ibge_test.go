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

// newIbgeRouter builds a minimal Chi router for the IBGE handler.
func newIbgeRouter(h *handlers.IbgeHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/ibge/municipio/{ibge}", h.GetMunicipio)
	r.Get("/v1/ibge/estados", h.GetEstados)
	r.Get("/v1/ibge/cnae/{codigo}", h.GetCNAE)
	return r
}

// mockIBGE starts a test server that returns the given body and status code.
// It returns an *handlers.IbgeHandler whose HTTP client points at the test server.
func mockIBGE(t *testing.T, statusCode int, body string) (*handlers.IbgeHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	h := handlers.NewIbgeHandlerWithClient(&http.Client{
		Transport: &ibgeRedirectTransport{base: srv.URL},
	})
	return h, srv
}

// ibgeRedirectTransport rewrites every request to point at the test server base URL
// while keeping the path intact.
type ibgeRedirectTransport struct {
	base string
}

func (t *ibgeRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

// TestGetMunicipio_OK verifies a happy-path lookup returns 200 with correct source and cost.
func TestGetMunicipio_OK(t *testing.T) {
	body := `{
		"id": 3550308,
		"nome": "São Paulo",
		"microrregiao": {"id": 35061, "nome": "São Paulo"}
	}`

	h, srv := mockIBGE(t, http.StatusOK, body)
	defer srv.Close()

	router := newIbgeRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/ibge/municipio/3550308", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "ibge_localidades" {
		t.Errorf("Source = %q, want ibge_localidades", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
}

// TestGetMunicipio_InvalidCode verifies that a short/invalid IBGE code gets 400.
func TestGetMunicipio_InvalidCode(t *testing.T) {
	h := handlers.NewIbgeHandler()
	router := newIbgeRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ibge/municipio/123", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for 3-digit code, got %d", rec.Code)
	}
}

// TestGetMunicipio_Upstream404 verifies that an upstream 404 is propagated as 404.
func TestGetMunicipio_Upstream404(t *testing.T) {
	h, srv := mockIBGE(t, http.StatusNotFound, `{}`)
	defer srv.Close()

	router := newIbgeRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/ibge/municipio/9999999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown municipality, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestGetEstados_OK verifies that a list of states is returned with total count.
func TestGetEstados_OK(t *testing.T) {
	body := `[{"id":35,"sigla":"SP","nome":"São Paulo"},{"id":33,"sigla":"RJ","nome":"Rio de Janeiro"}]`

	h, srv := mockIBGE(t, http.StatusOK, body)
	defer srv.Close()

	router := newIbgeRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/ibge/estados", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "ibge_localidades" {
		t.Errorf("Source = %q, want ibge_localidades", resp.Source)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}

	total, ok := resp.Data["total"].(float64)
	if !ok {
		t.Fatalf("Data[total] is not a number, got %T", resp.Data["total"])
	}
	if int(total) != 2 {
		t.Errorf("Data[total] = %v, want 2", total)
	}

	estados, ok := resp.Data["estados"].([]any)
	if !ok {
		t.Fatalf("Data[estados] is not a slice, got %T", resp.Data["estados"])
	}
	if len(estados) != 2 {
		t.Errorf("len(estados) = %d, want 2", len(estados))
	}
}

// TestGetEstados_UpstreamError verifies that an upstream 500 returns 502.
func TestGetEstados_UpstreamError(t *testing.T) {
	h, srv := mockIBGE(t, http.StatusInternalServerError, `{"error":"internal"}`)
	defer srv.Close()

	router := newIbgeRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/ibge/estados", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for upstream error, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestGetCNAE_OK verifies a happy-path CNAE lookup returns 200 with correct source.
func TestGetCNAE_OK(t *testing.T) {
	body := `{
		"id": "6201501",
		"descricao": "Desenvolvimento de programas de computador sob encomenda"
	}`

	h, srv := mockIBGE(t, http.StatusOK, body)
	defer srv.Close()

	router := newIbgeRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/ibge/cnae/6201501", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "ibge_cnae" {
		t.Errorf("Source = %q, want ibge_cnae", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
}

// TestGetCNAE_NotFound verifies that an upstream 404 is propagated as 404.
func TestGetCNAE_NotFound(t *testing.T) {
	h, srv := mockIBGE(t, http.StatusNotFound, `{}`)
	defer srv.Close()

	router := newIbgeRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/ibge/cnae/9999999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown CNAE, got %d: %s", rec.Code, rec.Body.String())
	}
}
