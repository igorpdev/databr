package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// pncpRedirectTransport rewrites requests to point at the test server.
type pncpRedirectTransport struct {
	base string
}

func (t *pncpRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

func mockPNCP(t *testing.T, statusCode int, body string) (*handlers.PNCPHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	h := handlers.NewPNCPHandlerWithClient(&http.Client{
		Transport: &pncpRedirectTransport{base: srv.URL},
	})
	return h, srv
}

func TestGetOrgaos_OK(t *testing.T) {
	body := `[{"codigoUnidade":"26246","nomeUnidade":"MINISTERIO DA EDUCACAO"}]`
	h, srv := mockPNCP(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/pncp/orgaos", h.GetOrgaos)

	req := httptest.NewRequest(http.MethodGet, "/v1/pncp/orgaos", nil)
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
	orgaos, _ := data["orgaos"].([]any)
	if len(orgaos) == 0 {
		t.Error("expected at least one orgao")
	}
}

func TestGetOrgaos_BadGateway(t *testing.T) {
	h, srv := mockPNCP(t, http.StatusInternalServerError, `{"error":"oops"}`)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/pncp/orgaos", h.GetOrgaos)

	req := httptest.NewRequest(http.MethodGet, "/v1/pncp/orgaos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", rec.Code)
	}
}
