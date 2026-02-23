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
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
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
	if resp.CostUSDC != "0.001" {
		t.Errorf("CostUSDC = %q, want 0.001", resp.CostUSDC)
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
	if resp.CostUSDC != "0.002" {
		t.Errorf("expected cost 0.002 (+0.001), got %s", resp.CostUSDC)
	}
}
