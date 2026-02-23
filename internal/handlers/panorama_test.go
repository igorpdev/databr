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
	"github.com/go-chi/chi/v5"
)

// panoramaStore implements handlers.SourceStore for panorama tests.
// It returns records keyed by source name for FindLatest.
type panoramaStore struct {
	bySource map[string][]domain.SourceRecord
	err      error
}

func (s *panoramaStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.bySource[source], nil
}

func (s *panoramaStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	return nil, nil
}

func (s *panoramaStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	return nil, nil
}

func newPanoramaRouter(h *handlers.PanoramaHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/economia/panorama", h.GetPanorama)
	return r
}

func TestPanorama_OK_AllSources(t *testing.T) {
	now := time.Now()
	store := &panoramaStore{
		bySource: map[string][]domain.SourceRecord{
			"bcb_selic":    {{Source: "bcb_selic", Data: map[string]any{"valor": "0.055"}, FetchedAt: now}},
			"ibge_ipca":    {{Source: "ibge_ipca", Data: map[string]any{"valor": "4.56"}, FetchedAt: now}},
			"ibge_pib":     {{Source: "ibge_pib", Data: map[string]any{"valor": "2.3"}, FetchedAt: now}},
			"bcb_ptax":     {{Source: "bcb_ptax", Data: map[string]any{"moeda": "USD", "cotacao_compra": 5.75}, FetchedAt: now}},
			"bcb_focus":    {{Source: "bcb_focus", Data: map[string]any{"ipca_12m": "4.5"}, FetchedAt: now}},
			"bcb_reservas": {{Source: "bcb_reservas", Data: map[string]any{"valor_bilhoes_usd": "350"}, FetchedAt: now}},
			"bcb_credito":  {{Source: "bcb_credito", Data: map[string]any{"valor_bilhoes_brl": "6100"}, FetchedAt: now}},
		},
	}

	h := handlers.NewPanoramaHandler(store)
	router := newPanoramaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/economia/panorama", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "panorama_economico" {
		t.Errorf("Source = %q, want panorama_economico", resp.Source)
	}
	if resp.CostUSDC != "0.010" {
		t.Errorf("CostUSDC = %q, want 0.010", resp.CostUSDC)
	}

	// Verify all 7 sources are present.
	expectedSources := []string{"bcb_selic", "ibge_ipca", "ibge_pib", "bcb_ptax", "bcb_focus", "bcb_reservas", "bcb_credito"}
	for _, src := range expectedSources {
		if resp.Data[src] == nil {
			t.Errorf("expected Data[%q] to be present", src)
		}
	}
}

func TestPanorama_PartialSources(t *testing.T) {
	now := time.Now()
	store := &panoramaStore{
		bySource: map[string][]domain.SourceRecord{
			"bcb_selic": {{Source: "bcb_selic", Data: map[string]any{"valor": "0.055"}, FetchedAt: now}},
			// All others missing
		},
	}

	h := handlers.NewPanoramaHandler(store)
	router := newPanoramaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/economia/panorama", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even with partial data, got %d", rec.Code)
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	// bcb_selic should have data.
	if resp.Data["bcb_selic"] == nil {
		t.Error("expected bcb_selic to have data")
	}
	// Missing sources should be nil.
	if resp.Data["ibge_ipca"] != nil {
		t.Error("expected ibge_ipca to be nil when not available")
	}
}

func TestPanorama_EmptyStore(t *testing.T) {
	store := &panoramaStore{bySource: map[string][]domain.SourceRecord{}}

	h := handlers.NewPanoramaHandler(store)
	router := newPanoramaRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/economia/panorama", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even with empty store, got %d", rec.Code)
	}
}
