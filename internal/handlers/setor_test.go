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

// setorCNPJFetcher implements handlers.CNPJFetcher for setor tests.
type setorCNPJFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *setorCNPJFetcher) FetchByCNPJ(ctx context.Context, cnpj string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

// setorStore implements handlers.SourceStore for setor tests.
type setorStore struct {
	filteredRecords map[string][]domain.SourceRecord // keyed by source
	err             error
}

func (s *setorStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	return nil, nil
}

func (s *setorStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	return nil, nil
}

func (s *setorStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	recs := s.filteredRecords[source]
	// Simulate filtering.
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

func newSetorRouter(h *handlers.SetorHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/empresas/{cnpj}/setor", h.GetSetor)
	return r
}

func TestSetor_OK_WithCNAE(t *testing.T) {
	cnpjFetcher := &setorCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":         "12345678000195",
				"razao_social": "EMPRESA XPTO LTDA",
				"cnae_fiscal":  "6201501",
			},
			FetchedAt: time.Now(),
		}},
	}
	store := &setorStore{
		filteredRecords: map[string][]domain.SourceRecord{
			"ibge_pib": {{
				Source: "ibge_pib",
				Data:   map[string]any{"cnae": "6201501", "valor": "100.5"},
			}},
			"b3_cotacoes": {},
		},
	}

	h := handlers.NewSetorHandler(cnpjFetcher, store)
	router := newSetorRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/setor", nil)
	req = x402pkg.InjectPrice(req, "0.007")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "setor_analise" {
		t.Errorf("Source = %q, want setor_analise", resp.Source)
	}
	if resp.CostUSDC != "0.007" {
		t.Errorf("CostUSDC = %q, want 0.007", resp.CostUSDC)
	}
	if resp.Data["cnae_fiscal"] != "6201501" {
		t.Errorf("cnae_fiscal = %v, want 6201501", resp.Data["cnae_fiscal"])
	}
	if resp.Data["publicly_traded"] != false {
		t.Errorf("publicly_traded = %v, want false", resp.Data["publicly_traded"])
	}
}

func TestSetor_PubliclyTraded(t *testing.T) {
	cnpjFetcher := &setorCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "12345678000195",
			Data: map[string]any{
				"cnpj":               "12345678000195",
				"cnae_fiscal":        "6201501",
				"codigo_natureza_juridica":  "2046",
			},
			FetchedAt: time.Now(),
		}},
	}
	store := &setorStore{
		filteredRecords: map[string][]domain.SourceRecord{
			"ibge_pib":    {},
			"b3_cotacoes": {},
		},
	}

	h := handlers.NewSetorHandler(cnpjFetcher, store)
	router := newSetorRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/setor", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Data["publicly_traded"] != true {
		t.Errorf("publicly_traded = %v, want true", resp.Data["publicly_traded"])
	}
}

func TestSetor_CNPJNotFound(t *testing.T) {
	cnpjFetcher := &setorCNPJFetcher{records: nil}
	store := &setorStore{filteredRecords: map[string][]domain.SourceRecord{}}

	h := handlers.NewSetorHandler(cnpjFetcher, store)
	router := newSetorRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/12345678000195/setor", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSetor_InvalidCNPJ(t *testing.T) {
	h := handlers.NewSetorHandler(&setorCNPJFetcher{}, &setorStore{filteredRecords: map[string][]domain.SourceRecord{}})
	router := newSetorRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/empresas/123/setor", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
