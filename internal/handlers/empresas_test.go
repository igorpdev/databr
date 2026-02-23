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

// stubCNPJFetcher implements handlers.CNPJFetcher using a fixed record.
type stubCNPJFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubCNPJFetcher) FetchByCNPJ(ctx context.Context, cnpj string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func newRouter(h *handlers.EmpresasHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/empresas/{cnpj}", h.GetEmpresa)
	r.Get("/v1/empresas/{cnpj}/socios", h.GetSocios)
	r.Get("/v1/empresas/{cnpj}/simples", h.GetSimples)
	return r
}

func TestEmpresasHandler_GetEmpresa_OK(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{
			{
				Source:    "cnpj",
				RecordKey: "12345678000195",
				Data: map[string]any{
					"cnpj":               "12345678000195",
					"razao_social":       "EMPRESA XPTO LTDA",
					"situacao_cadastral": "ATIVA",
					"uf":                 "SP",
					"municipio":          "SAO PAULO",
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Source != "cnpj" {
		t.Errorf("Source = %q, want %q", resp.Source, "cnpj")
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Error("Data must not be nil")
	}
	if resp.Data["cnpj"] != "12345678000195" {
		t.Errorf("Data[cnpj] = %v, want 12345678000195", resp.Data["cnpj"])
	}
}

func TestEmpresasHandler_GetEmpresa_NormalizeCNPJ(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{
			{
				Source:    "cnpj",
				RecordKey: "12345678000195",
				Data:      map[string]any{"cnpj": "12345678000195"},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	// CNPJ with dots and dashes (no slash — slash is a path separator and cannot
	// appear in a URL path segment, so real clients send digits-only or dot-dash format).
	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12.345.678-0001.95", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestEmpresasHandler_GetEmpresa_InvalidCNPJ(t *testing.T) {
	h := handlers.NewEmpresasHandler(&stubCNPJFetcher{})
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid CNPJ, got %d", rec.Code)
	}
}

func TestEmpresasHandler_GetEmpresa_ContentType(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{
			{Source: "cnpj", RecordKey: "12345678000195", Data: map[string]any{}, FetchedAt: time.Now()},
		},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestGetSocios_OK(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":         "12345678000195",
				"razao_social": "EMPRESA XPTO LTDA",
				"qsa": []any{
					map[string]any{
						"nome_socio":            "JOAO DA SILVA",
						"qualificacao_socio":    "Sócio-Administrador",
						"data_entrada_sociedade": "2020-01-01",
					},
				},
			},
			FetchedAt: time.Now(),
		}},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/socios", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cnpj" {
		t.Errorf("Source = %q, want cnpj", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	qsa, ok := resp.Data["qsa"]
	if !ok {
		t.Fatal("Data[qsa] must be present")
	}
	qsaSlice, ok := qsa.([]any)
	if !ok || len(qsaSlice) == 0 {
		t.Fatalf("Data[qsa] must be a non-empty slice, got %T", qsa)
	}
}

func TestGetSocios_NoQSA(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data:      map[string]any{"cnpj": "12345678000195", "razao_social": "EMPRESA SEM SOCIOS"},
			FetchedAt: time.Now(),
		}},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/socios", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when qsa is absent, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSocios_EmptyQSA(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data:      map[string]any{"cnpj": "12345678000195", "qsa": []any{}},
			FetchedAt: time.Now(),
		}},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/socios", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty qsa, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSocios_InvalidCNPJ(t *testing.T) {
	h := handlers.NewEmpresasHandler(&stubCNPJFetcher{})
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/123/socios", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid CNPJ, got %d", rec.Code)
	}
}

// ---------- GetSimples tests ----------

func TestGetSimples_OK(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":                     "12345678000195",
				"razao_social":             "EMPRESA XPTO LTDA",
				"opcao_pelo_simples":       true,
				"data_opcao_pelo_simples":  "2010-07-01",
				"data_exclusao_do_simples": nil,
				"opcao_pelo_mei":           false,
				"data_opcao_pelo_mei":      nil,
				"data_exclusao_do_mei":     nil,
			},
			FetchedAt: time.Now(),
		}},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/simples", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cnpj_simples" {
		t.Errorf("Source = %q, want cnpj_simples", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	if _, ok := resp.Data["opcao_pelo_simples"]; !ok {
		t.Error("Data must contain key 'opcao_pelo_simples'")
	}
	if _, ok := resp.Data["opcao_pelo_mei"]; !ok {
		t.Error("Data must contain key 'opcao_pelo_mei'")
	}
	if cnpjVal, _ := resp.Data["cnpj"].(string); cnpjVal != "12345678000195" {
		t.Errorf("Data[cnpj] = %q, want 12345678000195", cnpjVal)
	}
}

func TestGetSimples_NoSimples_Returns404(t *testing.T) {
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":         "12345678000195",
				"razao_social": "EMPRESA SEM SIMPLES",
			},
			FetchedAt: time.Now(),
		}},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/simples", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when simples/mei absent, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSimples_InvalidCNPJ_Returns400(t *testing.T) {
	h := handlers.NewEmpresasHandler(&stubCNPJFetcher{})
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/123/simples", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid CNPJ, got %d", rec.Code)
	}
}

func TestGetSimples_CNPJNotFound_Returns404(t *testing.T) {
	fetcher := &stubCNPJFetcher{records: nil}
	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/simples", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when CNPJ not found, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSimples_OnlySimples_OK(t *testing.T) {
	// mei key absent — should still return 200 because simples is present.
	fetcher := &stubCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj": "12345678000195",
				"simples": map[string]any{
					"optante": true,
				},
			},
			FetchedAt: time.Now(),
		}},
	}

	h := handlers.NewEmpresasHandler(fetcher)
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/simples", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
