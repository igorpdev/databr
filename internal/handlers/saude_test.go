package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

func newSaudeRouter(h *handlers.SaudeHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/saude/medicamentos/{registro}", h.GetMedicamento)
	return r
}

func TestSaudeHandler_GetMedicamento_OK(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{
			{
				Source:    "anvisa_medicamentos",
				RecordKey: "104400005",
				Data: map[string]any{
					"produto":            "ACERATUM",
					"empresa":            "CELLERA FARMACÊUTICA S.A.",
					"cnpj":               "33173097000274",
					"situacao":           "VÁLIDO",
					"data_vencimento":    "01/06/2025",
					"classe_terapeutica": "MEDICAMENTOS ATIVOS NA SECRECAO GORDUROSA",
					"principio_ativo":    "PERÓXIDO DE URÉIA",
					"categoria":          "SIMILAR",
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewSaudeHandler(store)
	r := newSaudeRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/medicamentos/104400005", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "anvisa_medicamentos" {
		t.Errorf("Source = %q, want %q", resp.Source, "anvisa_medicamentos")
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want %q", resp.CostUSDC, "0.003")
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	if produto, ok := resp.Data["produto"]; !ok || produto == "" {
		t.Error("Data must contain non-empty 'produto' field")
	}
}

func TestSaudeHandler_GetMedicamento_NotFound(t *testing.T) {
	store := &stubBCBStore{records: nil}

	h := handlers.NewSaudeHandler(store)
	r := newSaudeRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/medicamentos/999999999", nil)
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

func TestSaudeHandler_GetMedicamento_FormatContext(t *testing.T) {
	store := &stubBCBStore{
		records: []domain.SourceRecord{
			{
				Source:    "anvisa_medicamentos",
				RecordKey: "104400005",
				Data: map[string]any{
					"produto":  "ACERATUM",
					"situacao": "VÁLIDO",
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewSaudeHandler(store)
	r := newSaudeRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/saude/medicamentos/104400005?format=context", nil)
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
		t.Errorf("expected cost 0.005 (+0.002), got %s", resp.CostUSDC)
	}
}
