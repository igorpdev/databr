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

func titulosRecords() []domain.SourceRecord {
	return []domain.SourceRecord{
		{
			Source:    "tesouro_titulos",
			RecordKey: "Tesouro_SELIC_2027",
			Data: map[string]any{
				"nome":                   "Tesouro SELIC 2027",
				"vencimento":             "2027-03-01T00:00:00",
				"taxa_anual_compra":      0.0,
				"taxa_anual_resgate":     10.89,
				"preco_minimo":           149.77,
				"preco_unitario_resgate": 14891.39,
			},
			FetchedAt: time.Now(),
		},
		{
			Source:    "tesouro_titulos",
			RecordKey: "Tesouro_IPCA+_2035",
			Data: map[string]any{
				"nome":                   "Tesouro IPCA+ 2035",
				"vencimento":             "2035-05-15T00:00:00",
				"taxa_anual_compra":      6.45,
				"taxa_anual_resgate":     6.42,
				"preco_minimo":           34.57,
				"preco_unitario_resgate": 3456.78,
			},
			FetchedAt: time.Now(),
		},
	}
}

func TestTitulosHandler_GetTitulos_OK(t *testing.T) {
	store := &stubBCBStore{records: titulosRecords()}
	h := handlers.NewTitulosHandler(store)

	r := chi.NewRouter()
	r.Get("/v1/tesouro/titulos", h.GetTitulos)

	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/titulos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "tesouro_titulos" {
		t.Errorf("Source = %q, want tesouro_titulos", resp.Source)
	}
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Error("expected non-nil Data field")
	}
	titulos, ok := resp.Data["titulos"].([]any)
	if !ok {
		t.Fatalf("expected data.titulos to be []any, got %T", resp.Data["titulos"])
	}
	if len(titulos) != 2 {
		t.Errorf("expected 2 titulos, got %d", len(titulos))
	}
}

func TestTitulosHandler_GetTitulos_Empty(t *testing.T) {
	store := &stubBCBStore{records: nil}
	h := handlers.NewTitulosHandler(store)

	r := chi.NewRouter()
	r.Get("/v1/tesouro/titulos", h.GetTitulos)

	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/titulos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when no data, got %d", rec.Code)
	}
}

func TestTitulosHandler_GetTitulos_FormatContext(t *testing.T) {
	store := &stubBCBStore{records: titulosRecords()}
	h := handlers.NewTitulosHandler(store)

	r := chi.NewRouter()
	r.Get("/v1/tesouro/titulos", h.GetTitulos)

	req := httptest.NewRequest(http.MethodGet, "/v1/tesouro/titulos?format=context", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Context == "" {
		t.Error("expected non-empty Context when ?format=context")
	}
	if resp.Data != nil {
		t.Error("expected nil Data when ?format=context")
	}
	if resp.CostUSDC != "0.002" {
		t.Errorf("expected cost 0.002, got %s", resp.CostUSDC)
	}
}
