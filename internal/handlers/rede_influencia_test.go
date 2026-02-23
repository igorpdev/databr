package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

func newRedeInfluenciaRouter(h *handlers.RedeInfluenciaHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/rede/{cnpj}/influencia", h.GetRedeInfluencia)
	return r
}

func TestRedeInfluencia_OK_WithSocios(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data: map[string]any{
				"cnpj":         "11222333000181",
				"razao_social": "EMPRESA CENTRAL LTDA",
				"uf":           "SP",
				"cnae_fiscal":  "6201501",
				"qsa": []any{
					map[string]any{
						"nome_socio":            "JOAO DA SILVA",
						"cnpj_cpf_do_socio":     "12345678901",
						"qualificacao_socio":     "Socio-Administrador",
						"data_entrada_sociedade": "2020-01-15",
					},
					map[string]any{
						"nome_socio":            "MARIA OLIVEIRA",
						"cnpj_cpf_do_socio":     "98765432101",
						"qualificacao_socio":     "Socio",
						"data_entrada_sociedade": "2021-06-01",
					},
				},
			},
			FetchedAt: time.Now(),
		}},
	}
	store := &ddStore{records: nil}

	h := handlers.NewRedeInfluenciaHandler(cnpjFetcher, store)
	router := newRedeInfluenciaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/rede/11222333000181/influencia", nil)
	req = x402pkg.InjectPrice(req, "0.050")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "rede_influencia" {
		t.Errorf("Source = %q, want rede_influencia", resp.Source)
	}
	if resp.CostUSDC != "0.050" {
		t.Errorf("CostUSDC = %q, want 0.050", resp.CostUSDC)
	}

	// Check empresa_central.
	empresa, ok := resp.Data["empresa_central"].(map[string]any)
	if !ok {
		t.Fatalf("empresa_central not found in response")
	}
	if empresa["cnpj"] != "11222333000181" {
		t.Errorf("empresa_central.cnpj = %v, want 11222333000181", empresa["cnpj"])
	}
	if empresa["razao_social"] != "EMPRESA CENTRAL LTDA" {
		t.Errorf("empresa_central.razao_social = %v, want EMPRESA CENTRAL LTDA", empresa["razao_social"])
	}

	// Check socios.
	sociosRaw, ok := resp.Data["socios"].([]any)
	if !ok {
		t.Fatalf("socios not found or not a slice")
	}
	if len(sociosRaw) != 2 {
		t.Errorf("len(socios) = %d, want 2", len(sociosRaw))
	}

	// Check first socio name.
	if len(sociosRaw) > 0 {
		s0, ok := sociosRaw[0].(map[string]any)
		if !ok {
			t.Fatalf("socios[0] not a map")
		}
		if s0["nome"] != "JOAO DA SILVA" {
			t.Errorf("socios[0].nome = %v, want JOAO DA SILVA", s0["nome"])
		}
	}

	// Check estatisticas_rede.
	stats, ok := resp.Data["estatisticas_rede"].(map[string]any)
	if !ok {
		t.Fatalf("estatisticas_rede not found")
	}
	totalSocios, _ := stats["total_socios"].(float64)
	if totalSocios != 2 {
		t.Errorf("total_socios = %v, want 2", totalSocios)
	}
}

func TestRedeInfluencia_NoQSA(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data: map[string]any{
				"cnpj":         "11222333000181",
				"razao_social": "EMPRESA SEM QSA LTDA",
			},
			FetchedAt: time.Now(),
		}},
	}
	store := &ddStore{records: nil}

	h := handlers.NewRedeInfluenciaHandler(cnpjFetcher, store)
	router := newRedeInfluenciaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/rede/11222333000181/influencia", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Socios should be an empty slice.
	sociosRaw, ok := resp.Data["socios"].([]any)
	if !ok {
		t.Fatalf("socios not found or not a slice")
	}
	if len(sociosRaw) != 0 {
		t.Errorf("len(socios) = %d, want 0", len(sociosRaw))
	}

	// Empresas_conectadas should be empty.
	connected, ok := resp.Data["empresas_conectadas"].([]any)
	if !ok {
		t.Fatalf("empresas_conectadas not found or not a slice")
	}
	if len(connected) != 0 {
		t.Errorf("len(empresas_conectadas) = %d, want 0", len(connected))
	}

	// Estatisticas should show zeros.
	stats, ok := resp.Data["estatisticas_rede"].(map[string]any)
	if !ok {
		t.Fatalf("estatisticas_rede not found")
	}
	totalSocios, _ := stats["total_socios"].(float64)
	if totalSocios != 0 {
		t.Errorf("total_socios = %v, want 0", totalSocios)
	}
	totalConectadas, _ := stats["total_empresas_conectadas"].(float64)
	if totalConectadas != 0 {
		t.Errorf("total_empresas_conectadas = %v, want 0", totalConectadas)
	}
}

func TestRedeInfluencia_InvalidCNPJ(t *testing.T) {
	h := handlers.NewRedeInfluenciaHandler(
		&ddCNPJFetcher{},
		&ddStore{},
	)
	router := newRedeInfluenciaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/rede/123/influencia", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRedeInfluencia_CompanyNotFound(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{records: nil, err: nil} // no records, no error
	store := &ddStore{records: nil}

	h := handlers.NewRedeInfluenciaHandler(cnpjFetcher, store)
	router := newRedeInfluenciaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/rede/11222333000181/influencia", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRedeInfluencia_FetchError(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{err: errors.New("minhareceita unavailable")}
	store := &ddStore{records: nil}

	h := handlers.NewRedeInfluenciaHandler(cnpjFetcher, store)
	router := newRedeInfluenciaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/rede/11222333000181/influencia", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}
