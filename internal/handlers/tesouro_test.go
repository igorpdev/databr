package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

type stubSICONFIFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubSICONFIFetcher) FetchRREO(ctx context.Context, uf string, ano, periodo int) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func TestTesouroHandler_GetRREO_OK(t *testing.T) {
	fetcher := &stubSICONFIFetcher{
		records: []domain.SourceRecord{{
			Source:    "tesouro_siconfi",
			RecordKey: "SP_2024_1",
			Data:      map[string]any{"ente": "São Paulo", "uf": "SP", "an_exercicio": 2024},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewTesouroHandler(fetcher)
	r := chi.NewRouter()
	r.Get("/v1/tesouro/rreo", h.GetRREO)

	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/rreo?uf=SP&ano=2024&periodo=1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "tesouro_siconfi" {
		t.Errorf("Source = %q, want tesouro_siconfi", resp.Source)
	}
}

func TestTesouroHandler_GetRREO_MissingUF(t *testing.T) {
	h := handlers.NewTesouroHandler(&stubSICONFIFetcher{})
	r := chi.NewRouter()
	r.Get("/v1/tesouro/rreo", h.GetRREO)

	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/rreo", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// siconfiRedirectTransport rewrites requests to point at the test server.
type siconfiRedirectTransport struct {
	base string
}

func (t *siconfiRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

func mockSICONFI(t *testing.T, statusCode int, body string) (*handlers.TesouroHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	h := handlers.NewTesouroHandlerWithClient(nil, &http.Client{
		Transport: &siconfiRedirectTransport{base: srv.URL},
	})
	return h, srv
}

func newTesouroRouter(h *handlers.TesouroHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/tesouro/entes", h.GetEntes)
	r.Get("/v1/tesouro/rgf", h.GetRGF)
	r.Get("/v1/tesouro/dca", h.GetDCA)
	return r
}

func TestGetEntes_OK(t *testing.T) {
	body := `{"items":[{"co_ibge":"3550308","no_ente":"São Paulo"}],"hasMore":false,"limit":50,"offset":0,"count":1}`
	h, srv := mockSICONFI(t, http.StatusOK, body)
	defer srv.Close()

	router := newTesouroRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/entes", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	data := resp["data"].(map[string]any)
	entes, _ := data["entes"].([]any)
	if len(entes) == 0 {
		t.Error("expected at least one ente")
	}
}

func TestGetRGF_MissingUF(t *testing.T) {
	h := handlers.NewTesouroHandlerWithClient(nil, &http.Client{})
	router := newTesouroRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/rgf", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rec.Code)
	}
}

func TestGetRGF_OK(t *testing.T) {
	body := `{"items":[{"co_tipo_demonstrativo":"RGF","no_uf":"SP"}],"hasMore":false,"limit":50,"offset":0,"count":1}`
	h, srv := mockSICONFI(t, http.StatusOK, body)
	defer srv.Close()

	router := newTesouroRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/rgf?uf=SP", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetDCA_OK(t *testing.T) {
	body := `{"items":[{"co_tipo_demonstrativo":"DCA","an_exercicio":2023}],"hasMore":false,"limit":50,"offset":0,"count":1}`
	h, srv := mockSICONFI(t, http.StatusOK, body)
	defer srv.Close()

	router := newTesouroRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/dca", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
}
