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

// newIPEARouter builds a minimal Chi router for the IPEA handler.
func newIPEARouter(h *handlers.IPEAHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/ipea/serie/{codigo}", h.GetSerie)
	return r
}

// ipeaRedirectTransport rewrites every request to point at the test server
// while keeping the original path and query string intact.
type ipeaRedirectTransport struct {
	target string // httptest server URL, e.g. "http://127.0.0.1:PORT"
}

func (t *ipeaRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request and rewrite only the scheme and host so the path and
	// raw query (including OData $filter with spaces) are preserved verbatim.
	newReq := req.Clone(req.Context())
	newReq.URL.Scheme = "http"
	newReq.URL.Host = t.target[len("http://"):]
	return http.DefaultTransport.RoundTrip(newReq)
}

// mockIPEA starts a test server using the provided handler function and returns
// an *handlers.IPEAHandler whose HTTP client points at that server.
func mockIPEA(t *testing.T, handlerFunc http.HandlerFunc) (*handlers.IPEAHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handlerFunc)
	h := handlers.NewIPEAHandlerWithClient(&http.Client{
		Transport: &ipeaRedirectTransport{target: srv.URL},
	})
	return h, srv
}

// TestGetSerie_OK verifies a happy-path request returns 200 with correct envelope fields.
func TestGetSerie_OK(t *testing.T) {
	body := `{"value":[{"SERCODIGO":"BM12_TJOVER12","VALDATA":"2026-01-01T00:00:00-03:00","VALVALOR":1.16,"NIVNOME":"","TERCODIGO":""}]}`

	h, srv := mockIPEA(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	})
	defer srv.Close()

	router := newIPEARouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/ipea/serie/BM12_TJOVER12", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Source != "ipea_BM12_TJOVER12" {
		t.Errorf("Source = %q, want ipea_BM12_TJOVER12", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}

	total, ok := resp.Data["total"].(float64)
	if !ok {
		t.Fatalf("Data[total] is not a number, got %T", resp.Data["total"])
	}
	if int(total) != 1 {
		t.Errorf("Data[total] = %v, want 1", total)
	}

	serie, ok := resp.Data["serie"].(string)
	if !ok {
		t.Fatalf("Data[serie] is not a string, got %T", resp.Data["serie"])
	}
	if serie != "BM12_TJOVER12" {
		t.Errorf("Data[serie] = %q, want BM12_TJOVER12", serie)
	}

	valores, ok := resp.Data["valores"].([]any)
	if !ok {
		t.Fatalf("Data[valores] is not a slice, got %T", resp.Data["valores"])
	}
	if len(valores) != 1 {
		t.Errorf("len(valores) = %d, want 1", len(valores))
	}
}

// TestGetSerie_UpstreamError verifies that an upstream 500 returns 502.
func TestGetSerie_UpstreamError(t *testing.T) {
	h, srv := mockIPEA(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	})
	defer srv.Close()

	router := newIPEARouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/ipea/serie/BM12_TJOVER12", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for upstream error, got %d: %s", rec.Code, rec.Body.String())
	}
}

// mockIPEASimple is a convenience wrapper around mockIPEA for simple status+body mocks.
func mockIPEASimple(t *testing.T, statusCode int, body string) (*handlers.IPEAHandler, *httptest.Server) {
	t.Helper()
	return mockIPEA(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	})
}

func TestGetBusca_OK(t *testing.T) {
	body := `{"value":[{"SERCODIGO":"PRECOS12_IPCA12","SERNOME":"IPCA - variação mensal"}]}`
	h, srv := mockIPEASimple(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/ipea/busca", h.GetBusca)

	req := httptest.NewRequest(http.MethodGet, "/v1/ipea/busca?q=IPCA", nil)
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
	series, _ := data["series"].([]any)
	if len(series) == 0 {
		t.Error("expected at least one series")
	}
}

func TestGetBusca_ShortQuery(t *testing.T) {
	h := handlers.NewIPEAHandler()
	r := chi.NewRouter()
	r.Get("/v1/ipea/busca", h.GetBusca)

	req := httptest.NewRequest(http.MethodGet, "/v1/ipea/busca?q=I", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rec.Code)
	}
}

func TestGetTemas_OK(t *testing.T) {
	body := `{"value":[{"TEMCODIGO":1,"TEMNOME":"Macroeconomia"}]}`
	h, srv := mockIPEASimple(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/ipea/temas", h.GetTemas)

	req := httptest.NewRequest(http.MethodGet, "/v1/ipea/temas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestGetSerie_NotFound verifies that an empty value array returns 404.
func TestGetSerie_NotFound(t *testing.T) {
	h, srv := mockIPEA(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"value":[]}`))
	})
	defer srv.Close()

	router := newIPEARouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/ipea/serie/SERIES_INEXISTENTE", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty value array, got %d: %s", rec.Code, rec.Body.String())
	}
}
