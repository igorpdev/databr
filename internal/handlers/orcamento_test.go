package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func TestOrcamento_GetDespesas_OK(t *testing.T) {
	body := `[{"codigoOrgaoSuperior":"26000","nomeOrgaoSuperior":"MEC","despesaEmpenhada":1000000}]`
	srv := mockTCUUpstream(t, 200, body)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewOrcamentoHandlerWithClient(client, "test-key")
	r := chi.NewRouter()
	r.Get("/v1/orcamento/despesas", h.GetDespesas)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/despesas?ano=2025", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["source"] != "siafi_despesas" {
		t.Errorf("source = %q, want siafi_despesas", resp["source"])
	}
}

func TestOrcamento_GetDespesas_MissingAno(t *testing.T) {
	h := handlers.NewOrcamentoHandler()
	r := chi.NewRouter()
	r.Get("/v1/orcamento/despesas", h.GetDespesas)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/despesas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestOrcamento_GetFuncionalProgramatica_OK(t *testing.T) {
	body := `[{"funcao":"12","subfuncao":"361","programa":"2080"}]`
	srv := mockTCUUpstream(t, 200, body)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewOrcamentoHandlerWithClient(client, "test-key")
	r := chi.NewRouter()
	r.Get("/v1/orcamento/funcional-programatica", h.GetFuncionalProgramatica)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/funcional-programatica?ano=2025", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["source"] != "siafi_funcional_programatica" {
		t.Errorf("source = %q, want siafi_funcional_programatica", resp["source"])
	}
}

func TestOrcamento_GetFuncionalProgramatica_MissingAno(t *testing.T) {
	h := handlers.NewOrcamentoHandler()
	r := chi.NewRouter()
	r.Get("/v1/orcamento/funcional-programatica", h.GetFuncionalProgramatica)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/funcional-programatica", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestOrcamento_GetDocumentos_OK(t *testing.T) {
	body := `[{"codigo":"789","fase":"Empenho"}]`
	srv := mockTCUUpstream(t, 200, body)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewOrcamentoHandlerWithClient(client, "test-key")
	r := chi.NewRouter()
	r.Get("/v1/orcamento/documentos", h.GetDocumentos)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/documentos?fase=1&pagina=1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["source"] != "siafi_documentos" {
		t.Errorf("source = %q, want siafi_documentos", resp["source"])
	}
}

func TestOrcamento_GetDocumento_OK(t *testing.T) {
	body := `{"codigo":"123456","fase":"Empenho","valor":50000}`
	srv := mockTCUUpstream(t, 200, body)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewOrcamentoHandlerWithClient(client, "test-key")
	r := chi.NewRouter()
	r.Get("/v1/orcamento/documento/{codigo}", h.GetDocumento)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/documento/123456", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["source"] != "siafi_documento" {
		t.Errorf("source = %q, want siafi_documento", resp["source"])
	}
}

func TestOrcamento_GetDocumento_NotFound(t *testing.T) {
	srv := mockTCUUpstream(t, 404, `{"error":"not found"}`)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewOrcamentoHandlerWithClient(client, "test-key")
	r := chi.NewRouter()
	r.Get("/v1/orcamento/documento/{codigo}", h.GetDocumento)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/documento/999999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrcamento_GetFavorecidos_OK(t *testing.T) {
	body := `[{"codigoPessoa":"33000167000101","nome":"EMPRESA TESTE"}]`
	srv := mockTCUUpstream(t, 200, body)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewOrcamentoHandlerWithClient(client, "test-key")
	r := chi.NewRouter()
	r.Get("/v1/orcamento/favorecidos", h.GetFavorecidos)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/favorecidos?documento=33000167000101&ano=2025&fase=1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["source"] != "siafi_favorecidos" {
		t.Errorf("source = %q, want siafi_favorecidos", resp["source"])
	}
}

func TestOrcamento_GetFavorecidos_MissingParams(t *testing.T) {
	h := handlers.NewOrcamentoHandler()
	r := chi.NewRouter()
	r.Get("/v1/orcamento/favorecidos", h.GetFavorecidos)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/favorecidos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestOrcamento_GetDespesas_BadGateway(t *testing.T) {
	srv := mockTCUUpstream(t, 500, `{"error":"internal"}`)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewOrcamentoHandlerWithClient(client, "test-key")
	r := chi.NewRouter()
	r.Get("/v1/orcamento/despesas", h.GetDespesas)

	req := httptest.NewRequest(http.MethodGet, "/v1/orcamento/despesas?ano=2025", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 502 {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}
