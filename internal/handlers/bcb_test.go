package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// bcbSGSRedirectTransport rewrites BCB SGS API requests to point at the test server.
type bcbSGSRedirectTransport struct {
	base string
}

func (t *bcbSGSRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

// mockBCBSGS starts a test server returning the given body/status and returns a
// BCBHandler whose HTTP client redirects all BCB SGS calls to that server.
func mockBCBSGS(t *testing.T, statusCode int, body string) (*handlers.BCBHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	client := &http.Client{
		Transport: &bcbSGSRedirectTransport{base: srv.URL},
	}
	store := &stubBCBStore{}
	h := handlers.NewBCBHandlerWithClient(store, client)
	return h, srv
}

type stubBCBStore struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubBCBStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *stubBCBStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	for _, r := range s.records {
		if r.Source == source && r.RecordKey == key {
			return &r, nil
		}
	}
	return nil, nil
}

func (s *stubBCBStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	var out []domain.SourceRecord
	needle := strings.ToUpper(jsonbValue)
	for _, r := range s.records {
		if r.Source != source {
			continue
		}
		v, _ := r.Data[jsonbKey].(string)
		if strings.Contains(strings.ToUpper(v), needle) {
			out = append(out, r)
		}
	}
	return out, s.err
}

func newBCBRouter(h *handlers.BCBHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/bcb/selic", h.GetSelic)
	r.Get("/v1/bcb/cambio/{moeda}", h.GetCambio)
	return r
}

func TestBCBHandler_GetSelic_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{
			{
				Source:    "bcb_selic",
				RecordKey: "20/02/2026",
				Data:      map[string]any{"data": "20/02/2026", "valor": "0.055131"},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewBCBHandler(store)
	r := newBCBRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/selic", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "bcb_selic" {
		t.Errorf("Source = %q, want bcb_selic", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
}

func TestBCBHandler_GetCambio_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{
			{
				Source:    "bcb_ptax",
				RecordKey: "USD_2026-02-20",
				Data: map[string]any{
					"moeda":          "USD",
					"cotacao_compra": 5.75,
					"cotacao_venda":  5.76,
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewBCBHandler(store)
	r := newBCBRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/cambio/USD", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestBCBHandler_GetCambio_NotFound(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewBCBHandler(store)
	r := newBCBRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/cambio/USD", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestBCBHandler_GetPIX_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "bcb_pix",
			RecordKey: "202501",
			Data:      map[string]any{"ano_mes": "202501", "qtd_transacoes": float64(5000000000)},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewBCBHandler(store)
	r := chi.NewRouter()
	r.Get("/v1/bcb/pix/estatisticas", h.GetPIX)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/pix/estatisticas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBCBHandler_GetCredito_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "bcb_credito",
			RecordKey: "01/01/2026",
			Data:      map[string]any{"data": "01/01/2026", "valor_bilhoes_brl": "6100.5"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewBCBHandler(store)
	r := chi.NewRouter()
	r.Get("/v1/bcb/credito", h.GetCredito)
	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/credito", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestBCBHandler_GetReservas_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "bcb_reservas",
			RecordKey: "01/01/2026",
			Data:      map[string]any{"data": "01/01/2026", "valor_bilhoes_usd": "350.2"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewBCBHandler(store)
	r := chi.NewRouter()
	r.Get("/v1/bcb/reservas", h.GetReservas)
	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/reservas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestBCBHandler_GetTaxasCredito_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{
			{
				Source:    "bcb_taxas_credito",
				RecordKey: "Crédito pessoal não consignado_2025-01",
				Data: map[string]any{
					"segmento":        "Pessoa Física",
					"modalidade":      "Crédito pessoal não consignado",
					"posicao":         "A vista",
					"data_referencia": "2025-01",
					"taxa_mensal":     7.23,
					"taxa_anual":      130.45,
				},
				FetchedAt: time.Now(),
			},
			{
				Source:    "bcb_taxas_credito",
				RecordKey: "Cartão de crédito total_2025-01",
				Data: map[string]any{
					"segmento":        "Pessoa Física",
					"modalidade":      "Cartão de crédito total",
					"posicao":         "A vista",
					"data_referencia": "2025-01",
					"taxa_mensal":     15.12,
					"taxa_anual":      432.18,
				},
				FetchedAt: time.Now(),
			},
		},
	}
	h := handlers.NewBCBHandler(store)
	r := chi.NewRouter()
	r.Get("/v1/bcb/taxas-credito", h.GetTaxasCredito)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/taxas-credito", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "bcb_taxas_credito" {
		t.Errorf("Source = %q, want bcb_taxas_credito", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
	taxas, ok := resp.Data["taxas"].([]any)
	if !ok {
		t.Fatalf("expected data.taxas to be []any, got %T", resp.Data["taxas"])
	}
	if len(taxas) != 2 {
		t.Errorf("expected 2 taxas, got %d", len(taxas))
	}
}

func TestBCBHandler_GetTaxasCredito_Empty(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewBCBHandler(store)
	r := chi.NewRouter()
	r.Get("/v1/bcb/taxas-credito", h.GetTaxasCredito)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/taxas-credito", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no data, got %d", rec.Code)
	}
}

// --- GetIndicadores tests ---

func newIndicadoresRouter(h *handlers.BCBHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/bcb/indicadores/{serie}", h.GetIndicadores)
	return r
}

func TestGetIndicadores_ByName(t *testing.T) {
	body := `[{"data":"21/02/2026","valor":"0.0551"}]`
	h, srv := mockBCBSGS(t, http.StatusOK, body)
	defer srv.Close()

	router := newIndicadoresRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/indicadores/cdi", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "bcb_sgs" {
		t.Errorf("Source = %q, want bcb_sgs", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	valores, ok := resp.Data["valores"].([]any)
	if !ok {
		t.Fatalf("Data[valores] is not a slice, got %T", resp.Data["valores"])
	}
	if len(valores) != 1 {
		t.Errorf("expected 1 valor, got %d", len(valores))
	}
	if resp.Data["codigo"] == nil {
		t.Error("Data[codigo] must be set")
	}
}

func TestGetIndicadores_ByCode(t *testing.T) {
	body := `[{"data":"21/02/2026","valor":"0.0551"}]`
	h, srv := mockBCBSGS(t, http.StatusOK, body)
	defer srv.Close()

	router := newIndicadoresRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/indicadores/12", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "bcb_sgs" {
		t.Errorf("Source = %q, want bcb_sgs", resp.Source)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	// Numeric code should produce a generic name
	serie, _ := resp.Data["serie"].(string)
	if serie == "" {
		t.Error("Data[serie] must be a non-empty string")
	}
}

func TestGetIndicadores_InvalidSerie(t *testing.T) {
	store := &stubBCBStore{}
	h := handlers.NewBCBHandler(store)
	router := newIndicadoresRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/indicadores/xyz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid serie, got %d: %s", rec.Code, rec.Body.String())
	}
}

// bcbOLINDARedirectTransport rewrites BCB OLINDA requests to the test server.
type bcbOLINDARedirectTransport struct {
	base string
}

func (t *bcbOLINDARedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

func mockBCBOLINDA(t *testing.T, statusCode int, body string) (*handlers.BCBHandler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	client := &http.Client{
		Transport: &bcbOLINDARedirectTransport{base: srv.URL},
	}
	h := handlers.NewBCBHandlerWithClient(nil, client)
	return h, srv
}

func TestGetCapitais_OK(t *testing.T) {
	body := `{"value":[{"IdrRegistroIed":1,"DataRegistro":"2026-01-15"}]}`
	h, srv := mockBCBOLINDA(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/bcb/capitais", h.GetCapitais)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/capitais", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	data, _ := resp["data"].(map[string]any)
	regs, _ := data["registros"].([]any)
	if len(regs) == 0 {
		t.Error("expected at least 1 registro")
	}
}

func TestGetSML_All_OK(t *testing.T) {
	body := `{"value":[{"Moeda":"Real","Data":"15/02/2026","Cotacao":1.25}]}`
	h, srv := mockBCBOLINDA(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/bcb/sml", h.GetSML)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/sml", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSML_PaisInvalido(t *testing.T) {
	h := handlers.NewBCBHandler(nil)
	r := chi.NewRouter()
	r.Get("/v1/bcb/sml", h.GetSML)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/sml?pais=invalido", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rec.Code)
	}
}

// --- GetIFData tests ---

func TestGetIFData_OK(t *testing.T) {
	body := `{"value":[{"CNPJ8":"00000000","NomeInstituicao":"Banco do Brasil S.A.","CodAtivo":1}]}`
	h, srv := mockBCBOLINDA(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/bcb/ifdata", h.GetIFData)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/ifdata?n=5", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["source"] != "bcb_ifdata" {
		t.Errorf("source = %q, want bcb_ifdata", resp["source"])
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("data must not be nil")
	}
	insts, _ := data["instituicoes"].([]any)
	if len(insts) == 0 {
		t.Error("expected at least 1 instituicao")
	}
	if data["total"] == nil {
		t.Error("expected total field in data")
	}
}

func TestGetIFData_UpstreamError(t *testing.T) {
	h, srv := mockBCBOLINDA(t, http.StatusInternalServerError, `{"error":"internal"}`)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/bcb/ifdata", h.GetIFData)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/ifdata", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- GetBaseMonetaria tests ---

func TestGetBaseMonetaria_OK(t *testing.T) {
	// The handler fetches M0 and M2 sequentially from the same base URL.
	// The mock server returns the same body for both requests.
	body := `[{"data":"01/12/2025","valor":"303339501"}]`
	h, srv := mockBCBSGS(t, http.StatusOK, body)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/bcb/base-monetaria", h.GetBaseMonetaria)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/base-monetaria?n=3", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["source"] != "bcb_base_monetaria" {
		t.Errorf("source = %q, want bcb_base_monetaria", resp["source"])
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("data must not be nil")
	}
	if data["m0"] == nil {
		t.Error("expected m0 field in data")
	}
	if data["m2"] == nil {
		t.Error("expected m2 field in data")
	}
	m0, _ := data["m0"].([]any)
	if len(m0) == 0 {
		t.Error("expected at least 1 m0 entry")
	}
}

func TestGetBaseMonetaria_UpstreamError(t *testing.T) {
	h, srv := mockBCBSGS(t, http.StatusInternalServerError, `{"error":"internal"}`)
	defer srv.Close()

	r := chi.NewRouter()
	r.Get("/v1/bcb/base-monetaria", h.GetBaseMonetaria)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/base-monetaria", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBCBHandler_GetSelic_FormatContext(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "bcb_selic",
			RecordKey: "20/02/2026",
			Data:      map[string]any{"data": "20/02/2026", "valor": "0.055131"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewBCBHandler(store)
	r := newBCBRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/selic?format=context", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Context == "" {
		t.Error("expected non-empty Context field when ?format=context")
	}
	if resp.Data != nil {
		t.Error("expected nil Data when ?format=context")
	}
	if resp.CostUSDC != "0.005" {
		t.Errorf("expected cost 0.005 (+0.002), got %s", resp.CostUSDC)
	}
}
