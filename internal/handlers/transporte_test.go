package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// aeronaveRecords returns a set of test SourceRecords for anac_rab.
func aeronaveRecords() []domain.SourceRecord {
	now := time.Now()
	return []domain.SourceRecord{
		{
			Source:    "anac_rab",
			RecordKey: "PR-ABC",
			Data: map[string]any{
				"marca":         "PR-ABC",
				"proprietarios": "JOAO DA SILVA - 123.456.789-00 - 100%",
				"operador":      "EMPRESA AEREA LTDA",
				"uf":            "SP",
				"uf_operador":   "SP",
				"fabricante":    "ROBINSON",
				"modelo":        "R44 II",
				"ano_fabricacao": "2010",
				"tp_operacao":   "PRIVADA",
				"data_matricula": "2010-06-15",
			},
			FetchedAt: now,
		},
		{
			Source:    "anac_rab",
			RecordKey: "PT-XYZ",
			Data: map[string]any{
				"marca":         "PT-XYZ",
				"proprietarios": "MARIA SOUZA - 00.000.000/0001-00 - 100%",
				"operador":      "TAM LINHAS AEREAS",
				"uf":            "RJ",
				"uf_operador":   "RJ",
				"fabricante":    "CESSNA",
				"modelo":        "172C",
				"ano_fabricacao": "2005",
				"tp_operacao":   "PRIVADA",
				"data_matricula": "2005-03-20",
			},
			FetchedAt: now,
		},
	}
}

func newTransporteRouter(h *handlers.TransporteHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/transporte/aeronaves/{prefixo}", h.GetAeronave)
	r.Get("/v1/transporte/aeronaves", h.GetAeronaves)
	return r
}

// --- GetAeronave tests ---

func TestTransporteHandler_GetAeronave_OK(t *testing.T) {
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves/PR-ABC", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "anac_rab" {
		t.Errorf("Source = %q, want anac_rab", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	if fab, ok := resp.Data["fabricante"]; !ok || fab == "" {
		t.Error("Data must contain non-empty 'fabricante' field")
	}
}

func TestTransporteHandler_GetAeronave_CaseInsensitive(t *testing.T) {
	// Lowercase prefix should be normalized to uppercase and match.
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves/pr-abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (case-insensitive lookup), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTransporteHandler_GetAeronave_NotFound(t *testing.T) {
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves/ZZ-999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected non-empty 'error' field in 404 response")
	}
}

func TestTransporteHandler_GetAeronave_FormatContext(t *testing.T) {
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves/PR-ABC?format=context", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
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
		t.Errorf("expected cost 0.005 (+0.002 for context), got %s", resp.CostUSDC)
	}
}

// --- GetAeronaves tests ---

func TestTransporteHandler_GetAeronaves_OK(t *testing.T) {
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "anac_rab" {
		t.Errorf("Source = %q, want anac_rab", resp.Source)
	}
	if resp.CostUSDC != "0.005" {
		t.Errorf("CostUSDC = %q, want 0.005", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	total, ok := resp.Data["total"].(float64)
	if !ok || total == 0 {
		t.Error("expected non-zero 'total' in Data")
	}
}

func TestTransporteHandler_GetAeronaves_Empty(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no data, got %d", rec.Code)
	}
}

func TestTransporteHandler_GetAeronaves_FilterByUF(t *testing.T) {
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves?uf=SP", nil)
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
		if uf, ok := data["uf"].(string); ok && uf != "SP" {
			t.Errorf("filter returned uf %q, want SP", uf)
		}
	}
}

func TestTransporteHandler_GetAeronaves_FilterByFabricante(t *testing.T) {
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves?fabricante=CESSNA", nil)
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
		if fab, ok := data["fabricante"].(string); ok && fab != "CESSNA" {
			t.Errorf("filter returned fabricante %q, want CESSNA", fab)
		}
	}
}

func TestTransporteHandler_GetAeronaves_FilterByOperador(t *testing.T) {
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves?operador=TAM", nil)
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
	if len(records) == 0 {
		t.Error("expected at least one record matching operador=TAM")
	}
}

func TestTransporteHandler_GetAeronaves_FilterNotFound(t *testing.T) {
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves?uf=AM", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when filter matches nothing, got %d", rec.Code)
	}
}

func TestTransporteHandler_GetAeronaves_FormatContext(t *testing.T) {
	store := &stubBCBStore{records: aeronaveRecords()}
	h := handlers.NewTransporteHandler(store)
	r := newTransporteRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/aeronaves?format=context", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Context == "" {
		t.Error("expected non-empty Context when ?format=context")
	}
	if resp.CostUSDC != "0.007" {
		t.Errorf("expected cost 0.007 (0.005 + 0.002 for context), got %s", resp.CostUSDC)
	}
}
