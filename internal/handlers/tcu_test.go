package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func mockTCUUpstream(t *testing.T, statusCode int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
}

func TestTCU_GetAcordaos_OK(t *testing.T) {
	srv := mockTCUUpstream(t, 200, `[{"tipo":"ACÓRDÃO","numero":"123","ano":2026}]`)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewTCUHandlerWithClient(client)
	r := chi.NewRouter()
	r.Get("/v1/tcu/acordaos", h.GetAcordaos)

	req := httptest.NewRequest(http.MethodGet, "/v1/tcu/acordaos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["source"] != "tcu_acordaos" {
		t.Errorf("source = %q, want tcu_acordaos", resp["source"])
	}
}

func TestTCU_GetAcordaos_BadGateway(t *testing.T) {
	srv := mockTCUUpstream(t, 500, `{"error":"internal"}`)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewTCUHandlerWithClient(client)
	r := chi.NewRouter()
	r.Get("/v1/tcu/acordaos", h.GetAcordaos)

	req := httptest.NewRequest(http.MethodGet, "/v1/tcu/acordaos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 502 {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestTCU_GetCertidao_OK(t *testing.T) {
	srv := mockTCUUpstream(t, 200, `{"razaoSocial":"Empresa Teste","certidoes":[]}`)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewTCUHandlerWithClient(client)
	r := chi.NewRouter()
	r.Get("/v1/tcu/certidao/{cnpj}", h.GetCertidao)

	req := httptest.NewRequest(http.MethodGet, "/v1/tcu/certidao/33000167000101", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTCU_GetCertidao_InvalidCNPJ(t *testing.T) {
	h := handlers.NewTCUHandler()
	r := chi.NewRouter()
	r.Get("/v1/tcu/certidao/{cnpj}", h.GetCertidao)

	req := httptest.NewRequest(http.MethodGet, "/v1/tcu/certidao/123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTCU_GetInabilitados_OK(t *testing.T) {
	srv := mockTCUUpstream(t, 200, `[{"nome":"FULANO","CPF":"***123***"}]`)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewTCUHandlerWithClient(client)
	r := chi.NewRouter()
	r.Get("/v1/tcu/inabilitados", h.GetInabilitados)

	req := httptest.NewRequest(http.MethodGet, "/v1/tcu/inabilitados", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTCU_GetInabilitadoByCPF_OK(t *testing.T) {
	srv := mockTCUUpstream(t, 200, `{"nome":"FULANO","CPF":"12345678901"}`)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewTCUHandlerWithClient(client)
	r := chi.NewRouter()
	r.Get("/v1/tcu/inabilitados/{cpf}", h.GetInabilitadoByCPF)

	req := httptest.NewRequest(http.MethodGet, "/v1/tcu/inabilitados/12345678901", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTCU_GetInabilitadoByCPF_InvalidCPF(t *testing.T) {
	h := handlers.NewTCUHandler()
	r := chi.NewRouter()
	r.Get("/v1/tcu/inabilitados/{cpf}", h.GetInabilitadoByCPF)

	req := httptest.NewRequest(http.MethodGet, "/v1/tcu/inabilitados/123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTCU_GetContratos_OK(t *testing.T) {
	srv := mockTCUUpstream(t, 200, `[{"id":1,"fornecedor":"Empresa X"}]`)
	defer srv.Close()

	client := &http.Client{Transport: &transparenciaRedirectTransport{base: srv.URL}}
	h := handlers.NewTCUHandlerWithClient(client)
	r := chi.NewRouter()
	r.Get("/v1/tcu/contratos", h.GetContratos)

	req := httptest.NewRequest(http.MethodGet, "/v1/tcu/contratos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
