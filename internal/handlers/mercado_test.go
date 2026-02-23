package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

func newMercadoRouter(h *handlers.MercadoHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/mercado/acoes/{ticker}", h.GetAcoes)
	r.Get("/v1/mercado/fundos/{cnpj}", h.GetFundos)
	r.Get("/v1/mercado/fundos/{cnpj}/cotas", h.GetCotasByCNPJ)
	r.Get("/v1/mercado/fatos-relevantes", h.GetFatosRelevantes)
	r.Get("/v1/mercado/fatos-relevantes/{protocolo}", h.GetFatosById)
	return r
}

func TestMercadoHandler_GetAcoes_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "b3_cotacoes",
			RecordKey: "PETR4",
			Data:      map[string]any{"ticker": "PETR4", "preco_fechamento": 35.50},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/acoes/PETR4", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMercadoHandler_GetAcoes_NotFound(t *testing.T) {
	store := &stubBCBStore{}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/acoes/XXXX3", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestMercadoHandler_GetFundos_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{{
			Source:    "cvm_fundos",
			RecordKey: "12345678000195",
			Data:      map[string]any{"cnpj_fundo": "12345678000195", "denom_social": "FUNDO XYZ FIA"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/12345678000195", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMercadoHandler_GetFundos_NotFound(t *testing.T) {
	store := &stubBCBStore{}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/99999999000199", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func cotasRecords() []domain.SourceRecord {
	return []domain.SourceRecord{
		{
			Source:    "cvm_cotas",
			RecordKey: "11111111000111_2025-01-03",
			Data: map[string]any{
				"cnpj":          "11.111.111/0001-11",
				"cnpj_digits":   "11111111000111",
				"data":          "2025-01-03",
				"vl_quota":      "15.345678",
				"vl_patrimonio": "1100000.00",
				"captacao":      "100000.00",
				"resgate":       "0.00",
				"nr_cotistas":   "105",
			},
			FetchedAt: time.Now(),
		},
		{
			Source:    "cvm_cotas",
			RecordKey: "11111111000111_2025-01-02",
			Data: map[string]any{
				"cnpj":          "11.111.111/0001-11",
				"cnpj_digits":   "11111111000111",
				"data":          "2025-01-02",
				"vl_quota":      "15.234567",
				"vl_patrimonio": "1000000.00",
				"captacao":      "0.00",
				"resgate":       "0.00",
				"nr_cotistas":   "100",
			},
			FetchedAt: time.Now(),
		},
	}
}

func TestMercadoHandler_GetCotasByCNPJ_OK(t *testing.T) {
	store := &stubBCBStore{records: cotasRecords()}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/11111111000111/cotas", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cvm_cotas" {
		t.Errorf("Source = %q, want cvm_cotas", resp.Source)
	}
	if resp.CostUSDC != "0.005" {
		t.Errorf("CostUSDC = %q, want 0.005", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Error("expected non-nil Data field")
	}
	cotas, ok := resp.Data["cotas"].([]any)
	if !ok {
		t.Fatalf("expected data.cotas to be []any, got %T", resp.Data["cotas"])
	}
	if len(cotas) != 2 {
		t.Errorf("expected 2 cotas, got %d", len(cotas))
	}
}

func TestMercadoHandler_GetCotasByCNPJ_WithFormatting(t *testing.T) {
	// Test that CNPJs with partial formatting (digits and dashes) are normalized correctly.
	// Note: the full CNPJ format with slashes (11.111.111/0001-11) cannot be used in a path
	// segment; clients should strip the slash before the request or pass digits-only.
	store := &stubBCBStore{records: cotasRecords()}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	// Pass CNPJ with dots and dashes but without the slash (14-digit numeric equivalent)
	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/11111111000111/cotas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMercadoHandler_GetCotasByCNPJ_NotFound(t *testing.T) {
	store := &stubBCBStore{records: cotasRecords()}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/99999999000199/cotas", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func fatoRecords() []domain.SourceRecord {
	return []domain.SourceRecord{
		{
			Source:    "cvm_fatos",
			RecordKey: "001023IPE190120260001",
			Data: map[string]any{
				"cnpj":              "00.000.000/0001-91",
				"empresa":           "BANCO DO BRASIL S.A.",
				"codigo_cvm":        "1023",
				"categoria":         "Fato Relevante",
				"assunto":           "Payout 2026",
				"data_referencia":   "2026-01-19",
				"data_entrega":      "2026-01-19",
				"tipo_apresentacao": "AP - Apresentacao",
				"protocolo":         "001023IPE190120260001",
				"versao":            "1",
				"link_download":     "https://rad.cvm.gov.br/link1",
			},
			FetchedAt: time.Now(),
		},
		{
			Source:    "cvm_fatos",
			RecordKey: "009512IPE100220260003",
			Data: map[string]any{
				"cnpj":              "11.111.111/0001-11",
				"empresa":           "PETROBRAS S.A.",
				"codigo_cvm":        "9512",
				"categoria":         "Fato Relevante",
				"assunto":           "Descoberta de reservas",
				"data_referencia":   "2026-02-10",
				"data_entrega":      "2026-02-10",
				"tipo_apresentacao": "AP - Apresentacao",
				"protocolo":         "009512IPE100220260003",
				"versao":            "1",
				"link_download":     "https://rad.cvm.gov.br/link3",
			},
			FetchedAt: time.Now(),
		},
	}
}

func TestMercadoHandler_GetFatosRelevantes_OK(t *testing.T) {
	store := &stubBCBStore{records: fatoRecords()}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fatos-relevantes", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cvm_fatos" {
		t.Errorf("Source = %q, want cvm_fatos", resp.Source)
	}
	if resp.CostUSDC != "0.005" {
		t.Errorf("CostUSDC = %q, want 0.005", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Error("expected non-nil Data field")
	}
}

func TestMercadoHandler_GetFatosRelevantes_Empty(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fatos-relevantes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no data, got %d", rec.Code)
	}
}

func TestMercadoHandler_GetFatosRelevantes_FilterByCNPJ(t *testing.T) {
	store := &stubBCBStore{records: fatoRecords()}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	// Filter for Banco do Brasil only (by partial CNPJ)
	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fatos-relevantes?cnpj=00.000.000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	records, ok := resp.Data["records"].([]any)
	if !ok {
		t.Fatalf("expected data.records to be []any, got %T", resp.Data["records"])
	}
	for _, item := range records {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		data, ok := m["data"].(map[string]any)
		if !ok {
			continue
		}
		cnpjVal, _ := data["cnpj"].(string)
		if !strings.Contains(cnpjVal, "00.000.000") {
			t.Errorf("filter returned unexpected cnpj %q", cnpjVal)
		}
	}
}

func TestMercadoHandler_GetFatosRelevantes_FilterByCNPJ_NotFound(t *testing.T) {
	store := &stubBCBStore{records: fatoRecords()}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fatos-relevantes?cnpj=99.999.999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no match, got %d", rec.Code)
	}
}

func TestMercadoHandler_GetFatosById_OK(t *testing.T) {
	store := &stubBCBStore{records: fatoRecords()}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fatos-relevantes/001023IPE190120260001", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "cvm_fatos" {
		t.Errorf("Source = %q, want cvm_fatos", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Error("expected non-nil Data field")
	}
}

func TestMercadoHandler_GetFatosById_NotFound(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewMercadoHandler(store)
	r := newMercadoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fatos-relevantes/PROTOCOLO_NAO_EXISTE", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
