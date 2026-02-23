package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// Tests use municipality names (not IBGE codes) to avoid network calls.
// resolveIBGEToName only calls IBGE API for 6+ digit numeric strings,
// so using names like "Belem" bypasses the HTTP call entirely.

// riscoStore implements handlers.SourceStore for environmental risk tests.
type riscoStore struct {
	filteredRecords map[string][]domain.SourceRecord
	err             error
}

func (s *riscoStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	return nil, nil
}

func (s *riscoStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	return nil, nil
}

func (s *riscoStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
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

func newRiscoAmbientalRouter(h *handlers.RiscoAmbientalHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/ambiental/risco/{municipio}", h.GetRiscoAmbiental)
	return r
}

func TestRiscoAmbiental_LowRisk(t *testing.T) {
	store := &riscoStore{
		filteredRecords: map[string][]domain.SourceRecord{
			"inpe_deter": {{
				Source: "inpe_deter",
				Data:   map[string]any{"municipio": "Belem", "area_km2": 0.5},
			}},
			"inpe_prodes": {{
				Source: "inpe_prodes",
				Data:   map[string]any{"municipio": "Belem", "area_km2": 10.0},
			}},
		},
	}

	h := handlers.NewRiscoAmbientalHandler(store)
	router := newRiscoAmbientalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/risco/Belem", nil)
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
	if resp.Source != "risco_ambiental" {
		t.Errorf("Source = %q, want risco_ambiental", resp.Source)
	}
	if resp.CostUSDC != "0.007" {
		t.Errorf("CostUSDC = %q, want 0.007", resp.CostUSDC)
	}
	riskLevel, _ := resp.Data["risk_level"].(string)
	if riskLevel != "low" {
		t.Errorf("risk_level = %q, want low (1 alert)", riskLevel)
	}
	deterCount, _ := resp.Data["deter_count"].(float64)
	if deterCount != 1 {
		t.Errorf("deter_count = %v, want 1", deterCount)
	}
}

func TestRiscoAmbiental_MediumRisk(t *testing.T) {
	deterRecords := make([]domain.SourceRecord, 5)
	for i := range deterRecords {
		deterRecords[i] = domain.SourceRecord{
			Source: "inpe_deter",
			Data:   map[string]any{"municipio": "Belem", "area_km2": 0.5},
		}
	}
	store := &riscoStore{
		filteredRecords: map[string][]domain.SourceRecord{
			"inpe_deter":  deterRecords,
			"inpe_prodes": {},
		},
	}

	h := handlers.NewRiscoAmbientalHandler(store)
	router := newRiscoAmbientalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/risco/Belem", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	riskLevel, _ := resp.Data["risk_level"].(string)
	if riskLevel != "medium" {
		t.Errorf("risk_level = %q, want medium (5 alerts)", riskLevel)
	}
}

func TestRiscoAmbiental_HighRisk(t *testing.T) {
	deterRecords := make([]domain.SourceRecord, 15)
	for i := range deterRecords {
		deterRecords[i] = domain.SourceRecord{
			Source: "inpe_deter",
			Data:   map[string]any{"municipio": "Belem", "area_km2": 1.2},
		}
	}
	store := &riscoStore{
		filteredRecords: map[string][]domain.SourceRecord{
			"inpe_deter":  deterRecords,
			"inpe_prodes": {},
		},
	}

	h := handlers.NewRiscoAmbientalHandler(store)
	router := newRiscoAmbientalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/risco/Belem", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	riskLevel, _ := resp.Data["risk_level"].(string)
	if riskLevel != "high" {
		t.Errorf("risk_level = %q, want high (15 alerts)", riskLevel)
	}
}

func TestRiscoAmbiental_NoData(t *testing.T) {
	store := &riscoStore{
		filteredRecords: map[string][]domain.SourceRecord{
			"inpe_deter":  {},
			"inpe_prodes": {},
		},
	}

	h := handlers.NewRiscoAmbientalHandler(store)
	router := newRiscoAmbientalRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/risco/NenhumMunicipio", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	riskLevel, _ := resp.Data["risk_level"].(string)
	if riskLevel != "low" {
		t.Errorf("risk_level = %q, want low (no data)", riskLevel)
	}
}
