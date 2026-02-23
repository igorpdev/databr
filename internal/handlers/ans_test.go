package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// ansRedirectTransport rewrites requests so they point at the test mock server.
type ansRedirectTransport struct {
	base string
}

func (t *ansRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

// mockANSCSV is a minimal valid ANS CSV payload (UTF-8, semicolon-separated).
const mockANSCSV = `REGISTRO_OPERADORA;CNPJ;Razao_Social;Nome_Fantasia;Modalidade;Logradouro;Numero;Complemento;Bairro;Cidade;UF;CEP;DDD;Telefone;Fax;Endereco_eletronico;Representante;Cargo_Representante;Regiao_de_Comercializacao;Data_Registro_ANS
"419761";"19541931000125";"SAUDE EXAMPLE LTDA";"SAUDE EXAMPLE";"Medicina de Grupo";"RUA DAS FLORES";"100";;"CENTRO";"São Paulo";"SP";"01000000";"11";"11111111";;"contato@saude.com";"JOAO SILVA";"DIRETOR";4;"2015-05-19"
"421545";"22869997000153";"DENTE SAUDAVEL LTDA";;"Odontologia de Grupo";"RUA PRINCIPAL";"200";"SALA 10";"BAIRRO ALTO";"Rio de Janeiro";"RJ";"20000000";"21";"22222222";;"dente@saude.com";"MARIA SANTOS";"GERENTE";5;"2019-06-13"
"421421";"27452545000195";"COOPERSAUDE MG";"COOPERSAUDE";"Cooperativa Médica";"AVENIDA CENTRAL";"300";;"CENTRO";"Belo Horizonte";"MG";"30000000";"31";"33333333";;"coop@saude.com";"PEDRO COSTA";"REPRESENTANTE";4;"2018-10-09"
`

func mockANS(t *testing.T, statusCode int, body string) (*handlers.ANSHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	h := handlers.NewANSHandlerWithClient(&http.Client{
		Transport: &ansRedirectTransport{base: srv.URL},
	})
	return h, srv
}

func newANSRouter(h *handlers.ANSHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/saude/planos", h.GetPlanos)
	return r
}

func TestANSHandler_GetPlanos_OK(t *testing.T) {
	h, srv := mockANS(t, http.StatusOK, mockANSCSV)
	defer srv.Close()

	r := newANSRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/planos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["source"] != "ans_operadoras" {
		t.Errorf("source = %q, want %q", resp["source"], "ans_operadoras")
	}
	if resp["cost_usdc"] != "0.003" {
		t.Errorf("cost_usdc = %q, want 0.003", resp["cost_usdc"])
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("data field is not an object")
	}
	operadoras, ok := data["operadoras"].([]any)
	if !ok {
		t.Fatalf("data.operadoras is not an array")
	}
	if len(operadoras) != 3 {
		t.Errorf("expected 3 operadoras, got %d", len(operadoras))
	}
}

func TestANSHandler_GetPlanos_FilterUF(t *testing.T) {
	h, srv := mockANS(t, http.StatusOK, mockANSCSV)
	defer srv.Close()

	r := newANSRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/planos?uf=SP", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data := resp["data"].(map[string]any)
	operadoras := data["operadoras"].([]any)
	if len(operadoras) != 1 {
		t.Errorf("expected 1 SP operadora, got %d", len(operadoras))
	}
	op := operadoras[0].(map[string]any)
	if op["uf"] != "SP" {
		t.Errorf("expected UF=SP, got %q", op["uf"])
	}
}

func TestANSHandler_GetPlanos_FilterModalidade(t *testing.T) {
	h, srv := mockANS(t, http.StatusOK, mockANSCSV)
	defer srv.Close()

	r := newANSRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/planos?modalidade=Odontologia", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data := resp["data"].(map[string]any)
	operadoras := data["operadoras"].([]any)
	if len(operadoras) != 1 {
		t.Errorf("expected 1 odontology operadora, got %d", len(operadoras))
	}
}

func TestANSHandler_GetPlanos_LimitN(t *testing.T) {
	h, srv := mockANS(t, http.StatusOK, mockANSCSV)
	defer srv.Close()

	r := newANSRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/planos?n=2", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data := resp["data"].(map[string]any)
	operadoras := data["operadoras"].([]any)
	if len(operadoras) != 2 {
		t.Errorf("expected 2 operadoras with n=2, got %d", len(operadoras))
	}
}

func TestANSHandler_GetPlanos_BadGateway(t *testing.T) {
	h, srv := mockANS(t, http.StatusInternalServerError, `{"error":"oops"}`)
	defer srv.Close()

	r := newANSRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/planos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestANSHandler_GetPlanos_EmptyFilter(t *testing.T) {
	h, srv := mockANS(t, http.StatusOK, mockANSCSV)
	defer srv.Close()

	r := newANSRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/planos?uf=XX", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even for empty result, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data := resp["data"].(map[string]any)
	operadoras := data["operadoras"].([]any)
	if len(operadoras) != 0 {
		t.Errorf("expected 0 operadoras for XX UF, got %d", len(operadoras))
	}
}
