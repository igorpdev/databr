package handlers_test

import (
	"context"
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

type stubDataJudSearcher struct {
	records       []domain.SourceRecord
	err           error
	numberRecords []domain.SourceRecord
	numberErr     error
}

func (s *stubDataJudSearcher) Search(ctx context.Context, documento string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *stubDataJudSearcher) SearchByNumber(ctx context.Context, numero string) ([]domain.SourceRecord, error) {
	return s.numberRecords, s.numberErr
}

func TestJudicialHandler_GetProcessos_OK(t *testing.T) {
	searcher := &stubDataJudSearcher{
		records: []domain.SourceRecord{{
			Source:    "datajud_cnj",
			RecordKey: "0001234-56.2023.8.26.0001",
			Data:      map[string]any{"numeroProcesso": "0001234-56.2023.8.26.0001", "tribunal": "TJSP"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewJudicialHandler(searcher)
	r := chi.NewRouter()
	r.Get("/v1/judicial/processos/{doc}", h.GetProcessos)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processos/12345678909", nil)
	req = x402pkg.InjectPrice(req, "0.015")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CostUSDC != "0.015" {
		t.Errorf("CostUSDC = %q, want 0.015", resp.CostUSDC)
	}
}

func TestJudicialHandler_GetProcessos_NotFound(t *testing.T) {
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	r := chi.NewRouter()
	r.Get("/v1/judicial/processos/{doc}", h.GetProcessos)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processos/99999999999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestJudicialHandler_GetProcesso_OK(t *testing.T) {
	searcher := &stubDataJudSearcher{
		numberRecords: []domain.SourceRecord{{
			Source:    "datajud_cnj",
			RecordKey: "00008323520184013202",
			Data: map[string]any{
				"numeroProcesso": "00008323520184013202",
				"tribunal":       "TRF1",
				"classe":         map[string]any{"nome": "Recurso Inominado Cível"},
			},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewJudicialHandler(searcher)
	r := chi.NewRouter()
	r.Get("/v1/judicial/processo/{numero}", h.GetProcesso)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processo/0000832-35.2018.4.01.3202", nil)
	req = x402pkg.InjectPrice(req, "0.010")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "datajud_cnj" {
		t.Errorf("Source = %q, want datajud_cnj", resp.Source)
	}
	if resp.CostUSDC != "0.010" {
		t.Errorf("CostUSDC = %q, want 0.010", resp.CostUSDC)
	}
}

func TestJudicialHandler_GetProcesso_InvalidFormat(t *testing.T) {
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	r := chi.NewRouter()
	r.Get("/v1/judicial/processo/{numero}", h.GetProcesso)

	// No hyphens or dots — invalid CNJ format
	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processo/12345678901234567890", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestJudicialHandler_GetProcesso_NotFound(t *testing.T) {
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	r := chi.NewRouter()
	r.Get("/v1/judicial/processo/{numero}", h.GetProcesso)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/processo/0000832-35.2018.4.01.3202", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// judicialStore implements handlers.SourceStore for STF/STJ tests.
type judicialStore struct {
	bySource map[string][]domain.SourceRecord
	err      error
}

func (s *judicialStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.bySource[source], nil
}

func (s *judicialStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	return nil, nil
}

func (s *judicialStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	return nil, nil
}

func TestJudicialHandler_GetSTF_OK(t *testing.T) {
	store := &judicialStore{
		bySource: map[string][]domain.SourceRecord{
			"stf_decisoes": {
				{
					Source:    "stf_decisoes",
					RecordKey: "ADI-1234",
					Data: map[string]any{
						"id":             "ADI-1234",
						"classe":         "ADI",
						"numero":         "1234",
						"relator":        "Min. Fulano de Tal",
						"orgao_julgador": "Tribunal Pleno",
						"ementa":         "Ementa da decisao.",
					},
					FetchedAt: time.Now(),
				},
				{
					Source:    "stf_decisoes",
					RecordKey: "RE-567890",
					Data: map[string]any{
						"id":             "RE-567890",
						"classe":         "RE",
						"numero":         "567890",
						"relator":        "Min. Ciclana Silva",
						"orgao_julgador": "Segunda Turma",
						"ementa":         "Ementa sobre recurso.",
					},
					FetchedAt: time.Now(),
				},
			},
		},
	}
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	h.SetStore(store)

	r := chi.NewRouter()
	r.Get("/v1/judicial/stf", h.GetSTF)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/stf", nil)
	req = x402pkg.InjectPrice(req, "0.010")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "stf_decisoes" {
		t.Errorf("Source = %q, want stf_decisoes", resp.Source)
	}
	if resp.CostUSDC != "0.010" {
		t.Errorf("CostUSDC = %q, want 0.010", resp.CostUSDC)
	}
	total, _ := resp.Data["total"].(float64)
	if total != 2 {
		t.Errorf("total = %v, want 2", total)
	}
	decisoes, ok := resp.Data["decisoes"].([]any)
	if !ok || len(decisoes) != 2 {
		t.Errorf("decisoes length = %v, want 2", len(decisoes))
	}
}

func TestJudicialHandler_GetSTJ_OK(t *testing.T) {
	store := &judicialStore{
		bySource: map[string][]domain.SourceRecord{
			"stj_decisoes": {
				{
					Source:    "stj_decisoes",
					RecordKey: "REsp 1.234.567/SP",
					Data: map[string]any{
						"processo":        "REsp 1.234.567/SP",
						"classe":          "REsp",
						"relator":         "Min. Beltrano",
						"orgao_julgador":  "Terceira Turma",
						"ementa":          "Ementa recurso especial.",
						"acordao":         "Vistos e relatados...",
					},
					FetchedAt: time.Now(),
				},
			},
		},
	}
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	h.SetStore(store)

	r := chi.NewRouter()
	r.Get("/v1/judicial/stj", h.GetSTJ)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/stj", nil)
	req = x402pkg.InjectPrice(req, "0.010")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "stj_decisoes" {
		t.Errorf("Source = %q, want stj_decisoes", resp.Source)
	}
	total, _ := resp.Data["total"].(float64)
	if total != 1 {
		t.Errorf("total = %v, want 1", total)
	}
}

func TestJudicialHandler_GetSTF_NoStore(t *testing.T) {
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	// No SetStore — store is nil

	r := chi.NewRouter()
	r.Get("/v1/judicial/stf", h.GetSTF)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/stf", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestJudicialHandler_GetSTF_Empty(t *testing.T) {
	store := &judicialStore{
		bySource: map[string][]domain.SourceRecord{},
	}
	h := handlers.NewJudicialHandler(&stubDataJudSearcher{})
	h.SetStore(store)

	r := chi.NewRouter()
	r.Get("/v1/judicial/stf", h.GetSTF)

	req := httptest.NewRequest(http.MethodGet, "/v1/judicial/stf", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
