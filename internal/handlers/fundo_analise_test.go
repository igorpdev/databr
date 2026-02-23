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
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// fundoStore implements handlers.SourceStore for fund analysis tests.
type fundoStore struct {
	bySource        map[string][]domain.SourceRecord
	byKey           map[string]*domain.SourceRecord // keyed by "source:key"
	filteredRecords map[string][]domain.SourceRecord
	err             error
}

func (s *fundoStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.bySource[source], nil
}

func (s *fundoStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	k := source + ":" + key
	if rec, ok := s.byKey[k]; ok {
		return rec, nil
	}
	return nil, nil
}

func (s *fundoStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	recs := s.filteredRecords[source]
	var out []domain.SourceRecord
	needle := strings.ToUpper(jsonbValue)
	for _, r := range recs {
		v, _ := r.Data[jsonbKey].(string)
		if strings.Contains(strings.ToUpper(v), needle) {
			out = append(out, r)
		}
	}
	return out, nil
}

func newFundoAnaliseRouter(h *handlers.FundoAnaliseHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/mercado/fundos/{cnpj}/analise", h.GetFundoAnalise)
	return r
}

func TestFundoAnalise_OK(t *testing.T) {
	now := time.Now()
	store := &fundoStore{
		byKey: map[string]*domain.SourceRecord{
			"cvm_fundos:12345678000195": {
				Source:    "cvm_fundos",
				RecordKey: "12345678000195",
				Data: map[string]any{
					"cnpj":           "12345678000195",
					"nome":           "FUNDO TESTE FI",
					"classe":         "Renda Fixa",
					"patrimonio":     1000000.0,
				},
				FetchedAt: now,
			},
		},
		bySource: map[string][]domain.SourceRecord{
			"bcb_selic": {{Source: "bcb_selic", Data: map[string]any{"valor": "13.75"}, FetchedAt: now}},
			"ibge_ipca": {{Source: "ibge_ipca", Data: map[string]any{"valor": "4.56"}, FetchedAt: now}},
		},
		filteredRecords: map[string][]domain.SourceRecord{
			"cvm_cotas": {
				{Source: "cvm_cotas", Data: map[string]any{"cnpj": "12345678000195", "valor_cota": "110.0", "data": "2026-02-01"}},
				{Source: "cvm_cotas", Data: map[string]any{"cnpj": "12345678000195", "valor_cota": "100.0", "data": "2025-02-01"}},
			},
		},
	}

	h := handlers.NewFundoAnaliseHandler(store)
	router := newFundoAnaliseRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/12345678000195/analise", nil)
	req = x402pkg.InjectPrice(req, "0.010")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "fundo_analise" {
		t.Errorf("Source = %q, want fundo_analise", resp.Source)
	}
	if resp.CostUSDC != "0.010" {
		t.Errorf("CostUSDC = %q, want 0.010", resp.CostUSDC)
	}

	// Fund data should be present.
	fundo, _ := resp.Data["fundo"].(map[string]any)
	if fundo == nil {
		t.Fatal("expected fundo data to be present")
	}
	if fundo["nome"] != "FUNDO TESTE FI" {
		t.Errorf("fundo.nome = %v, want FUNDO TESTE FI", fundo["nome"])
	}

	// Performance: 100 -> 110 = 10% nominal return.
	nominalReturn, _ := resp.Data["nominal_return"].(float64)
	if nominalReturn != 10 {
		t.Errorf("nominal_return = %v, want 10", nominalReturn)
	}

	// Real return = 10 - 4.56 = 5.44
	realReturn, _ := resp.Data["real_return"].(float64)
	if realReturn < 5.43 || realReturn > 5.45 {
		t.Errorf("real_return = %v, want ~5.44", realReturn)
	}

	// vs CDI = 10 - 13.75 = -3.75
	vsCDI, _ := resp.Data["vs_cdi"].(float64)
	if vsCDI < -3.76 || vsCDI > -3.74 {
		t.Errorf("vs_cdi = %v, want ~-3.75", vsCDI)
	}
}

func TestFundoAnalise_NotFound(t *testing.T) {
	store := &fundoStore{
		byKey:           map[string]*domain.SourceRecord{},
		bySource:        map[string][]domain.SourceRecord{},
		filteredRecords: map[string][]domain.SourceRecord{},
	}

	h := handlers.NewFundoAnaliseHandler(store)
	router := newFundoAnaliseRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/12345678000195/analise", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFundoAnalise_NoCotas(t *testing.T) {
	now := time.Now()
	store := &fundoStore{
		byKey: map[string]*domain.SourceRecord{
			"cvm_fundos:12345678000195": {
				Source:    "cvm_fundos",
				RecordKey: "12345678000195",
				Data:      map[string]any{"cnpj": "12345678000195", "nome": "FUNDO SEM COTAS"},
				FetchedAt: now,
			},
		},
		bySource: map[string][]domain.SourceRecord{
			"bcb_selic": {{Source: "bcb_selic", Data: map[string]any{"valor": "13.75"}, FetchedAt: now}},
			"ibge_ipca": {{Source: "ibge_ipca", Data: map[string]any{"valor": "4.56"}, FetchedAt: now}},
		},
		filteredRecords: map[string][]domain.SourceRecord{
			"cvm_cotas": {},
		},
	}

	h := handlers.NewFundoAnaliseHandler(store)
	router := newFundoAnaliseRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/12345678000195/analise", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	nominalReturn, _ := resp.Data["nominal_return"].(float64)
	if nominalReturn != 0 {
		t.Errorf("nominal_return = %v, want 0 (no cotas)", nominalReturn)
	}
}

func TestFundoAnalise_InvalidCNPJ(t *testing.T) {
	h := handlers.NewFundoAnaliseHandler(&fundoStore{
		byKey:           map[string]*domain.SourceRecord{},
		bySource:        map[string][]domain.SourceRecord{},
		filteredRecords: map[string][]domain.SourceRecord{},
	})
	router := newFundoAnaliseRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/mercado/fundos/123/analise", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
