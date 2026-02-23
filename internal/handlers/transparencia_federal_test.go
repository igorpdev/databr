package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// stubTransparenciaFetcher implements TransparenciaFetcher for tests.
type stubTransparenciaFetcher struct {
	contratosRecords  []domain.SourceRecord
	contratosErr      error
	servidoresRecords []domain.SourceRecord
	servidoresErr     error
	beneficiosRecords []domain.SourceRecord
	beneficiosErr     error
	cartoesRecords    []domain.SourceRecord
	cartoesErr        error
}

func (s *stubTransparenciaFetcher) FetchContratos(_ context.Context, _, _ string) ([]domain.SourceRecord, error) {
	return s.contratosRecords, s.contratosErr
}

func (s *stubTransparenciaFetcher) FetchServidores(_ context.Context, _ string) ([]domain.SourceRecord, error) {
	return s.servidoresRecords, s.servidoresErr
}

func (s *stubTransparenciaFetcher) FetchBolsaFamilia(_ context.Context, _, _ string) ([]domain.SourceRecord, error) {
	return s.beneficiosRecords, s.beneficiosErr
}

func (s *stubTransparenciaFetcher) FetchCartoes(_ context.Context, _, _, _ string) ([]domain.SourceRecord, error) {
	return s.cartoesRecords, s.cartoesErr
}

func newTransparenciaFederalRouter(h *handlers.TransparenciaFederalHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/transparencia/contratos", h.GetContratos)
	r.Get("/v1/transparencia/servidores", h.GetServidores)
	r.Get("/v1/transparencia/beneficios", h.GetBolsaFamilia)
	r.Get("/v1/transparencia/cartoes", h.GetCartoes)
	return r
}

// --- GetContratos ---

func TestTransparenciaFederal_GetContratos_OK(t *testing.T) {
	stub := &stubTransparenciaFetcher{
		contratosRecords: []domain.SourceRecord{{
			Source:    "cgu_contratos",
			RecordKey: "26000",
			Data: map[string]any{
				"orgao":     "26000",
				"contratos": []any{map[string]any{"numero": "001/2025"}},
				"total":     1,
			},
			FetchedAt: time.Now().UTC(),
		}},
	}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/contratos?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cgu_contratos" {
		t.Errorf("Source = %q, want cgu_contratos", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
}

func TestTransparenciaFederal_GetContratos_WithOptionalCNPJ(t *testing.T) {
	stub := &stubTransparenciaFetcher{
		contratosRecords: []domain.SourceRecord{{
			Source:    "cgu_contratos",
			RecordKey: "26000",
			Data:      map[string]any{"orgao": "26000", "contratos": []any{}, "total": 0},
			FetchedAt: time.Now().UTC(),
		}},
	}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	// orgao required + optional cnpj filter — both params accepted without 400
	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/contratos?orgao=26000&cnpj=33000167000101", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (stub returns record), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTransparenciaFederal_GetContratos_MissingOrgao(t *testing.T) {
	h := handlers.NewTransparenciaFederalHandler(&stubTransparenciaFetcher{})
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/contratos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetContratos_FetcherError(t *testing.T) {
	stub := &stubTransparenciaFetcher{contratosErr: errors.New("API key not set")}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/contratos?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetContratos_Empty(t *testing.T) {
	stub := &stubTransparenciaFetcher{contratosRecords: []domain.SourceRecord{}}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/contratos?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- GetServidores ---

func TestTransparenciaFederal_GetServidores_OK(t *testing.T) {
	stub := &stubTransparenciaFetcher{
		servidoresRecords: []domain.SourceRecord{{
			Source:    "cgu_servidores",
			RecordKey: "26000",
			Data: map[string]any{
				"orgao":      "26000",
				"servidores": []any{map[string]any{"nome": "JOAO SILVA"}},
				"total":      1,
			},
			FetchedAt: time.Now().UTC(),
		}},
	}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/servidores?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cgu_servidores" {
		t.Errorf("Source = %q, want cgu_servidores", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
}

func TestTransparenciaFederal_GetServidores_MissingOrgao(t *testing.T) {
	h := handlers.NewTransparenciaFederalHandler(&stubTransparenciaFetcher{})
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/servidores", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetServidores_FetcherError(t *testing.T) {
	stub := &stubTransparenciaFetcher{servidoresErr: errors.New("upstream error")}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/servidores?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetServidores_Empty(t *testing.T) {
	stub := &stubTransparenciaFetcher{servidoresRecords: []domain.SourceRecord{}}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/servidores?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- GetBolsaFamilia ---

func TestTransparenciaFederal_GetBolsaFamilia_OK(t *testing.T) {
	stub := &stubTransparenciaFetcher{
		beneficiosRecords: []domain.SourceRecord{{
			Source:    "cgu_beneficios",
			RecordKey: "3550308_202501",
			Data: map[string]any{
				"municipio_ibge": "3550308",
				"mes":            "202501",
				"beneficios":     []any{map[string]any{"cpfFormatado": "***.***.***-**"}},
				"total":          1,
			},
			FetchedAt: time.Now().UTC(),
		}},
	}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/beneficios?municipio_ibge=3550308&mes=202501", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cgu_beneficios" {
		t.Errorf("Source = %q, want cgu_beneficios", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
}

func TestTransparenciaFederal_GetBolsaFamilia_DefaultMonth(t *testing.T) {
	// When mes is omitted, handler should default to previous month.
	stub := &stubTransparenciaFetcher{
		beneficiosRecords: []domain.SourceRecord{{
			Source:    "cgu_beneficios",
			RecordKey: "3550308_202501",
			Data: map[string]any{
				"municipio_ibge": "3550308",
				"mes":            "202501",
				"beneficios":     []any{},
				"total":          0,
			},
			FetchedAt: time.Now().UTC(),
		}},
	}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/beneficios?municipio_ibge=3550308", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should reach the fetcher (not 400) — response may be 200 or 404 depending on stub
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected non-400 when mes is omitted (default should apply), got 400: %s", rec.Body.String())
	}
}

func TestTransparenciaFederal_GetBolsaFamilia_MissingMunicipio(t *testing.T) {
	h := handlers.NewTransparenciaFederalHandler(&stubTransparenciaFetcher{})
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/beneficios", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetBolsaFamilia_FetcherError(t *testing.T) {
	stub := &stubTransparenciaFetcher{beneficiosErr: errors.New("api error")}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/beneficios?municipio_ibge=3550308&mes=202501", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetBolsaFamilia_Empty(t *testing.T) {
	stub := &stubTransparenciaFetcher{beneficiosRecords: []domain.SourceRecord{}}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/beneficios?municipio_ibge=3550308&mes=202501", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- GetCartoes ---

func TestTransparenciaFederal_GetCartoes_OK(t *testing.T) {
	stub := &stubTransparenciaFetcher{
		cartoesRecords: []domain.SourceRecord{{
			Source:    "cgu_cartoes",
			RecordKey: "26000_2026-01-01_2026-01-31",
			Data: map[string]any{
				"orgao":      "26000",
				"de":         "2026-01-01",
				"ate":        "2026-01-31",
				"transacoes": []any{map[string]any{"id": 1, "valorTransacao": 150.50}},
				"total":      1,
			},
			FetchedAt: time.Now().UTC(),
		}},
	}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/cartoes?orgao=26000&de=2026-01-01&ate=2026-01-31", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cgu_cartoes" {
		t.Errorf("Source = %q, want cgu_cartoes", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
}

func TestTransparenciaFederal_GetCartoes_MissingOrgao(t *testing.T) {
	h := handlers.NewTransparenciaFederalHandler(&stubTransparenciaFetcher{})
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/cartoes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetCartoes_FetcherError(t *testing.T) {
	stub := &stubTransparenciaFetcher{cartoesErr: errors.New("api error")}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/cartoes?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetCartoes_Empty(t *testing.T) {
	stub := &stubTransparenciaFetcher{cartoesRecords: []domain.SourceRecord{}}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/cartoes?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- GetCEAF ---

// transparenciaRedirectTransport rewrites requests to point at a test server.
type transparenciaRedirectTransport struct {
	base string
}

func (t *transparenciaRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

func mockTransparenciaHTTP(t *testing.T, statusCode int, body string) (*handlers.TransparenciaFederalHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewTransparenciaFederalHandlerWithClient(&stubTransparenciaFetcher{}, client, "test-api-key")
	return h, srv
}

func TestTransparenciaFederal_GetCEAF_OK(t *testing.T) {
	body := `[{"cnpj":"00000000000191","nome":"Entidade Teste"}]`
	h, srv := mockTransparenciaHTTP(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/transparencia/ceaf/{cnpj}", h.GetCEAF)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/ceaf/00000000000191", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["source"] != "cgu_ceaf" {
		t.Errorf("source = %q, want cgu_ceaf", resp["source"])
	}
}

func TestTransparenciaFederal_GetCEAF_InvalidCNPJ(t *testing.T) {
	h := handlers.NewTransparenciaFederalHandlerWithClient(&stubTransparenciaFetcher{}, &http.Client{}, "key")
	r := chi.NewRouter()
	r.Get("/v1/transparencia/ceaf/{cnpj}", h.GetCEAF)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/ceaf/123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetCEAF_NotFound(t *testing.T) {
	h, srv := mockTransparenciaHTTP(t, http.StatusNotFound, `{}`)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/transparencia/ceaf/{cnpj}", h.GetCEAF)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/ceaf/00000000000191", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", rec.Code)
	}
}

// --- GetViagens ---

func TestTransparenciaFederal_GetViagens_OK(t *testing.T) {
	body := `[{"id":1,"destino":"Brasília","valor":1500.00}]`
	h, srv := mockTransparenciaHTTP(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/transparencia/viagens", h.GetViagens)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/viagens?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["source"] != "cgu_viagens" {
		t.Errorf("source = %q, want cgu_viagens", resp["source"])
	}
}

func TestTransparenciaFederal_GetViagens_BadGateway(t *testing.T) {
	h, srv := mockTransparenciaHTTP(t, http.StatusInternalServerError, `{"error":"oops"}`)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/transparencia/viagens", h.GetViagens)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/viagens?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", rec.Code)
	}
}

func TestTransparenciaFederal_GetCartoes_DefaultDates(t *testing.T) {
	// When de/ate omitted, defaults should apply and fetcher is called.
	stub := &stubTransparenciaFetcher{
		cartoesRecords: []domain.SourceRecord{{
			Source:    "cgu_cartoes",
			RecordKey: "26000",
			Data:      map[string]any{"orgao": "26000", "transacoes": []any{}, "total": 0},
			FetchedAt: time.Now().UTC(),
		}},
	}
	h := handlers.NewTransparenciaFederalHandler(stub)
	r := newTransparenciaFederalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transparencia/cartoes?orgao=26000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should reach the fetcher (not 400). stub returns 1 record → 200.
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected non-400 when de/ate omitted (defaults should apply), got 400: %s", rec.Body.String())
	}
}
