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

func newTransparenciaFederalRouter(h *handlers.TransparenciaFederalHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/transparencia/contratos", h.GetContratos)
	r.Get("/v1/transparencia/servidores", h.GetServidores)
	r.Get("/v1/transparencia/beneficios", h.GetBolsaFamilia)
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
