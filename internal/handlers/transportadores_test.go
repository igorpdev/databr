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

// newTransportadoresRouter wires both ANTT endpoints onto a test Chi router.
func newTransportadoresRouter(h *handlers.TransportadoresHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/transporte/transportadores/{rntrc}", h.GetTransportador)
	r.Get("/v1/transporte/transportadores", h.GetTransportadoresByCNPJ)
	return r
}

// sampleTransportadorRecords returns two ANTT RNTRC SourceRecords for use in tests.
func sampleTransportadorRecords() []domain.SourceRecord {
	return []domain.SourceRecord{
		{
			Source:    "antt_rntrc",
			RecordKey: "050085788",
			Data: map[string]any{
				"nome":            "EMPRESA DE TRANSPORTE TESTE LTDA",
				"rntrc":           "050085788",
				"situacao":        "ATIVO",
				"categoria":       "ETC",
				"cpf_cnpj":        "11.193.322/0001-10",
				"cpf_cnpj_digits": "11193322000110",
				"cep":             "14095-290",
				"municipio":       "RIBEIRAO PRETO",
				"uf":              "SP",
				"equiparado":      "Sim",
				"data_cadastro":   "23/05/2017",
				"data_situacao":   "23/10/2024",
			},
			FetchedAt: time.Now(),
		},
		{
			Source:    "antt_rntrc",
			RecordKey: "012345678",
			Data: map[string]any{
				"nome":            "JOAO DA SILVA TRANSPORTE",
				"rntrc":           "012345678",
				"situacao":        "ATIVO",
				"categoria":       "TAC",
				"cpf_cnpj":        "123.456.789-09",
				"cpf_cnpj_digits": "12345678909",
				"cep":             "01310-100",
				"municipio":       "SAO PAULO",
				"uf":              "SP",
				"equiparado":      "Nao",
				"data_cadastro":   "15/03/2010",
				"data_situacao":   "01/01/2025",
			},
			FetchedAt: time.Now(),
		},
	}
}

// --- GetTransportador (by RNTRC) ---

func TestTransportadoresHandler_GetTransportador_OK(t *testing.T) {
	store := &stubBCBStore{records: sampleTransportadorRecords()}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores/050085788", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "antt_rntrc" {
		t.Errorf("Source = %q, want antt_rntrc", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	if nome, ok := resp.Data["nome"].(string); !ok || nome == "" {
		t.Error("Data must contain non-empty 'nome' field")
	}
}

func TestTransportadoresHandler_GetTransportador_LeadingZeroPad(t *testing.T) {
	// The store has RecordKey "050085788"; request arrives without leading zero padding.
	store := &stubBCBStore{records: sampleTransportadorRecords()}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	// "50085788" (8 digits) should be padded to "050085788" (9 digits).
	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores/50085788", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after zero-padding, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTransportadoresHandler_GetTransportador_NotFound(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores/000000001", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected non-empty 'error' field in 404 response")
	}
}

func TestTransportadoresHandler_GetTransportador_InvalidRNTRC(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	// Non-numeric RNTRC should return 400.
	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores/ABC123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric rntrc, got %d", rec.Code)
	}
}

func TestTransportadoresHandler_GetTransportador_FormatContext(t *testing.T) {
	store := &stubBCBStore{records: sampleTransportadorRecords()}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores/050085788?format=context", nil)
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
		t.Error("expected non-empty Context when ?format=context")
	}
	if resp.Data != nil {
		t.Error("expected nil Data when ?format=context")
	}
	if resp.CostUSDC != "0.005" {
		t.Errorf("expected cost 0.005 (+0.002 for context), got %s", resp.CostUSDC)
	}
}

// --- GetTransportadoresByCNPJ (by CNPJ query param) ---

func TestTransportadoresHandler_GetTransportadoresByCNPJ_OK(t *testing.T) {
	store := &stubBCBStore{records: sampleTransportadorRecords()}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	// Formatted CNPJ — handler must strip non-digits before DB lookup.
	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores?cnpj=11.193.322/0001-10", nil)
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
	if resp.Source != "antt_rntrc" {
		t.Errorf("Source = %q, want antt_rntrc", resp.Source)
	}
	if resp.CostUSDC != "0.005" {
		t.Errorf("CostUSDC = %q, want 0.005", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
}

func TestTransportadoresHandler_GetTransportadoresByCNPJ_DigitsOnly(t *testing.T) {
	store := &stubBCBStore{records: sampleTransportadorRecords()}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	// CNPJ as digits only — should still work.
	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores?cnpj=11193322000110", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for digits-only CNPJ, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTransportadoresHandler_GetTransportadoresByCNPJ_NotFound(t *testing.T) {
	store := &stubBCBStore{records: sampleTransportadorRecords()}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores?cnpj=99999999000199", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown CNPJ, got %d", rec.Code)
	}
}

func TestTransportadoresHandler_GetTransportadoresByCNPJ_MissingParam(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	// No ?cnpj= parameter.
	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when ?cnpj= missing, got %d", rec.Code)
	}
}

func TestTransportadoresHandler_GetTransportadoresByCNPJ_ResponseHasList(t *testing.T) {
	store := &stubBCBStore{records: sampleTransportadorRecords()}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores?cnpj=11193322000110", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Data == nil {
		t.Fatal("expected non-nil Data")
	}
	records, ok := resp.Data["records"].([]any)
	if !ok {
		t.Fatalf("expected Data[records] to be []any, got %T", resp.Data["records"])
	}
	if len(records) == 0 {
		t.Error("expected at least 1 record in response list")
	}
	if resp.Data["total"] == nil {
		t.Error("expected Data[total] to be present")
	}
}

func TestTransportadoresHandler_GetTransportadoresByCNPJ_FormatContext(t *testing.T) {
	store := &stubBCBStore{records: sampleTransportadorRecords()}
	h := handlers.NewTransportadoresHandler(store)
	r := newTransportadoresRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/transporte/transportadores?cnpj=11193322000110&format=context", nil)
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
