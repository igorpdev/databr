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

// newEnderecoRouter builds a minimal Chi router for the endereco handler,
// optionally overriding the ViaCEP base URL via a custom http.Client transport.
func newEnderecoRouter(h *handlers.EnderecoHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/endereco/{cep}", h.GetEndereco)
	return r
}

// mockViaCEP starts a test server that returns the given body and status code.
// It returns a *handlers.EnderecoHandler whose HTTP client points at the test server.
func mockViaCEP(t *testing.T, statusCode int, body string) (*handlers.EnderecoHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	h := handlers.NewEnderecoHandlerWithClient(&http.Client{
		Transport: &redirectTransport{base: srv.URL},
	})
	return h, srv
}

// redirectTransport rewrites every request to point at the test server base URL
// while keeping the path (so /ws/{cep}/json/ still reaches the mock).
type redirectTransport struct {
	base string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the scheme+host with the test server's URL.
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

// TestEnderecoHandler_ValidCEP_Returns200 verifies a happy-path lookup.
func TestEnderecoHandler_ValidCEP_Returns200(t *testing.T) {
	viaCEPBody := `{
		"cep": "01310-100",
		"logradouro": "Avenida Paulista",
		"complemento": "",
		"bairro": "Bela Vista",
		"localidade": "São Paulo",
		"uf": "SP",
		"ibge": "3550308"
	}`

	h, srv := mockViaCEP(t, http.StatusOK, viaCEPBody)
	defer srv.Close()

	router := newEnderecoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/endereco/01310100", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "viacep" {
		t.Errorf("Source = %q, want viacep", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	if uf, _ := resp.Data["uf"].(string); uf != "SP" {
		t.Errorf("Data[uf] = %q, want SP", uf)
	}
}

// TestEnderecoHandler_CEPWithHyphen verifies that "01310-100" is accepted and normalised.
func TestEnderecoHandler_CEPWithHyphen_Returns200(t *testing.T) {
	viaCEPBody := `{"cep":"01310-100","logradouro":"Avenida Paulista","uf":"SP"}`

	h, srv := mockViaCEP(t, http.StatusOK, viaCEPBody)
	defer srv.Close()

	router := newEnderecoRouter(h)
	// Chi decodes path params, so we pass the hyphen-form.
	req := httptest.NewRequest(http.MethodGet, "/v1/endereco/01310-100", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestEnderecoHandler_InvalidFormat_Returns400 verifies that a short/invalid CEP gets 400.
func TestEnderecoHandler_InvalidFormat_Returns400(t *testing.T) {
	h := handlers.NewEnderecoHandler()
	router := newEnderecoRouter(h)

	for _, bad := range []string{"123", "abcdefgh", "0131010"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/endereco/"+bad, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("CEP %q: expected 400, got %d", bad, rec.Code)
		}
	}
}

// TestEnderecoHandler_NotFound_Returns404 verifies the ViaCEP {"erro":"true"} case.
func TestEnderecoHandler_NotFound_Returns404(t *testing.T) {
	h, srv := mockViaCEP(t, http.StatusOK, `{"erro": "true"}`)
	defer srv.Close()

	router := newEnderecoRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/endereco/00000000", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown CEP, got %d: %s", rec.Code, rec.Body.String())
	}
}
