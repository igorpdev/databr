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
)

// stubAmbientalStore satisfies SourceStore interface.
type stubAmbientalStore struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubAmbientalStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *stubAmbientalStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	for _, r := range s.records {
		if r.Source == source && r.RecordKey == key {
			return &r, nil
		}
	}
	return nil, nil
}

func TestAmbientalHandler_GetDesmatamento_OK(t *testing.T) {
	store := &stubAmbientalStore{
		records: []domain.SourceRecord{
			{
				Source:    "inpe_deter",
				RecordKey: "deter_amz.fid-abc123",
				Data: map[string]any{
					"area_km2":            0.1017,
					"municipio":           "obidos",
					"estado":              "PA",
					"data_deteccao":       "2024-01-14",
					"classe_desmatamento": "DESMATAMENTO_CR",
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewAmbientalHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/desmatamento", nil)
	rec := httptest.NewRecorder()
	h.GetDesmatamento(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "inpe_deter" {
		t.Errorf("Source = %q, want inpe_deter", resp.Source)
	}
	if resp.CostUSDC != "0.002" {
		t.Errorf("CostUSDC = %q, want 0.002", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Error("expected non-nil Data")
	}
}

func TestAmbientalHandler_GetDesmatamento_Empty(t *testing.T) {
	store := &stubAmbientalStore{records: []domain.SourceRecord{}}

	h := handlers.NewAmbientalHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/desmatamento", nil)
	rec := httptest.NewRecorder()
	h.GetDesmatamento(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAmbientalHandler_GetProdes_OK(t *testing.T) {
	store := &stubAmbientalStore{
		records: []domain.SourceRecord{
			{
				Source:    "inpe_prodes",
				RecordKey: "PA_2023",
				Data: map[string]any{
					"area_km2": 11568.0,
					"ano":      2023,
					"estado":   "PA",
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewAmbientalHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/prodes", nil)
	rec := httptest.NewRecorder()
	h.GetProdes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "inpe_prodes" {
		t.Errorf("Source = %q, want inpe_prodes", resp.Source)
	}
	if resp.CostUSDC != "0.002" {
		t.Errorf("CostUSDC = %q, want 0.002", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Error("expected non-nil Data")
	}
}

func TestAmbientalHandler_GetProdes_Empty(t *testing.T) {
	store := &stubAmbientalStore{records: []domain.SourceRecord{}}

	h := handlers.NewAmbientalHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/prodes", nil)
	rec := httptest.NewRecorder()
	h.GetProdes(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAmbientalHandler_GetDesmatamento_FormatContext(t *testing.T) {
	store := &stubAmbientalStore{
		records: []domain.SourceRecord{
			{
				Source:    "inpe_deter",
				RecordKey: "deter_amz.fid-abc123",
				Data: map[string]any{
					"area_km2": 0.1017,
					"estado":   "PA",
				},
				FetchedAt: time.Now(),
			},
		},
	}

	h := handlers.NewAmbientalHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/ambiental/desmatamento?format=context", nil)
	rec := httptest.NewRecorder()
	h.GetDesmatamento(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Context == "" {
		t.Error("expected non-empty Context field when ?format=context")
	}
	if resp.Data != nil {
		t.Error("expected nil Data when ?format=context")
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("expected cost 0.003 (+0.001), got %s", resp.CostUSDC)
	}
}
