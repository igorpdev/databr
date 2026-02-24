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

// empregoStore implements handlers.SourceStore for emprego tests.
// It filters records by source in FindLatest (unlike ddStore which returns all).
type empregoStore struct {
	records []domain.SourceRecord
	err     error
}

func (s *empregoStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	var out []domain.SourceRecord
	for _, r := range s.records {
		if r.Source == source {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *empregoStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	for _, r := range s.records {
		if r.Source == source && r.RecordKey == key {
			return &r, nil
		}
	}
	return nil, s.err
}

func (s *empregoStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	return s.FindLatest(ctx, source)
}

func newEmpregoRouter(h *handlers.EmpregoHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/emprego/caged", h.GetCAGED)
	r.Get("/v1/emprego/rais", h.GetRAIS)
	return r
}

// cagedRecord builds a CAGED source record with the given period and items.
// Items are stored as []any to simulate JSONB deserialization from PostgreSQL.
func cagedRecord(periodo string, items []map[string]any) domain.SourceRecord {
	// Convert to []any to simulate JSONB deserialization
	anyItems := make([]any, len(items))
	for i, item := range items {
		anyItems[i] = item
	}
	return domain.SourceRecord{
		Source:    "caged_emprego",
		RecordKey: periodo,
		Data: map[string]any{
			"periodo": periodo,
			"items":   anyItems,
			"total":   len(items),
		},
		FetchedAt: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
	}
}

// raisRecord builds a RAIS source record with the given year and items.
func raisRecord(ano string, items []map[string]any) domain.SourceRecord {
	anyItems := make([]any, len(items))
	for i, item := range items {
		anyItems[i] = item
	}
	return domain.SourceRecord{
		Source:    "rais_emprego",
		RecordKey: ano,
		Data: map[string]any{
			"periodo": ano,
			"items":   anyItems,
			"total":   len(items),
		},
		FetchedAt: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
	}
}

// --- CAGED Tests ---

func TestEmpregoHandler_GetCAGED_AllItems(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			cagedRecord("202501", []map[string]any{
				{"uf": "SP", "cnae_secao": "C", "admissoes": float64(145000), "desligamentos": float64(130000), "saldo": float64(15000)},
				{"uf": "RJ", "cnae_secao": "G", "admissoes": float64(50000), "desligamentos": float64(40000), "saldo": float64(10000)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/caged", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "caged_emprego" {
		t.Errorf("Source = %q, want caged_emprego", resp.Source)
	}

	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	total, _ := resp.Data["total"].(float64)
	if total != 2 {
		t.Errorf("expected total=2, got %v", total)
	}
}

func TestEmpregoHandler_GetCAGED_FilterByUF(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			cagedRecord("202501", []map[string]any{
				{"uf": "SP", "cnae_secao": "C", "admissoes": float64(145000), "desligamentos": float64(130000), "saldo": float64(15000)},
				{"uf": "RJ", "cnae_secao": "G", "admissoes": float64(50000), "desligamentos": float64(40000), "saldo": float64(10000)},
				{"uf": "SP", "cnae_secao": "G", "admissoes": float64(80000), "desligamentos": float64(70000), "saldo": float64(10000)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/caged?uf=SP", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items for SP, got %d", len(items))
	}
	total, _ := resp.Data["total"].(float64)
	if total != 2 {
		t.Errorf("expected total=2, got %v", total)
	}

	// Verify all items have uf=SP
	for i, item := range items {
		m, _ := item.(map[string]any)
		if m["uf"] != "SP" {
			t.Errorf("item[%d].uf = %v, want SP", i, m["uf"])
		}
	}
}

func TestEmpregoHandler_GetCAGED_FilterByUF_Lowercase(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			cagedRecord("202501", []map[string]any{
				{"uf": "SP", "admissoes": float64(100)},
				{"uf": "RJ", "admissoes": float64(50)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	// lowercase "sp" should match "SP"
	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/caged?uf=sp", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item for sp (case-insensitive), got %d", len(items))
	}
}

func TestEmpregoHandler_GetCAGED_FilterByMes(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			cagedRecord("202502", []map[string]any{
				{"uf": "SP", "admissoes": float64(200)},
			}),
			cagedRecord("202501", []map[string]any{
				{"uf": "SP", "admissoes": float64(100)},
				{"uf": "RJ", "admissoes": float64(50)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/caged?mes=202501", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Should have selected the 202501 period (2 items)
	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items for period 202501, got %d", len(items))
	}
	periodo, _ := resp.Data["periodo"].(string)
	if periodo != "202501" {
		t.Errorf("expected periodo=202501, got %v", periodo)
	}
}

func TestEmpregoHandler_GetCAGED_FilterByMes_NotFound(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			cagedRecord("202501", []map[string]any{
				{"uf": "SP", "admissoes": float64(100)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/caged?mes=202412", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEmpregoHandler_GetCAGED_NotFound(t *testing.T) {
	store := &empregoStore{records: nil}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/caged", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEmpregoHandler_GetCAGED_CombinedMesAndUF(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			cagedRecord("202502", []map[string]any{
				{"uf": "SP", "admissoes": float64(200)},
				{"uf": "RJ", "admissoes": float64(150)},
			}),
			cagedRecord("202501", []map[string]any{
				{"uf": "SP", "admissoes": float64(100)},
				{"uf": "RJ", "admissoes": float64(50)},
				{"uf": "MG", "admissoes": float64(75)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/caged?mes=202501&uf=RJ", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (RJ in 202501), got %d", len(items))
	}
	total, _ := resp.Data["total"].(float64)
	if total != 1 {
		t.Errorf("expected total=1, got %v", total)
	}
}

// --- RAIS Tests ---

func TestEmpregoHandler_GetRAIS_AllItems(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			raisRecord("2024", []map[string]any{
				{"uf": "SP", "cnae_secao": "C", "admissoes": float64(1000000), "desligamentos": float64(900000), "saldo": float64(100000)},
				{"uf": "RJ", "cnae_secao": "G", "admissoes": float64(500000), "desligamentos": float64(450000), "saldo": float64(50000)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/rais", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "rais_emprego" {
		t.Errorf("Source = %q, want rais_emprego", resp.Source)
	}

	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	total, _ := resp.Data["total"].(float64)
	if total != 2 {
		t.Errorf("expected total=2, got %v", total)
	}
}

func TestEmpregoHandler_GetRAIS_FilterByUF(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			raisRecord("2024", []map[string]any{
				{"uf": "SP", "admissoes": float64(1000000)},
				{"uf": "RJ", "admissoes": float64(500000)},
				{"uf": "SP", "admissoes": float64(800000)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/rais?uf=SP", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items for SP, got %d", len(items))
	}
	total, _ := resp.Data["total"].(float64)
	if total != 2 {
		t.Errorf("expected total=2, got %v", total)
	}
}

func TestEmpregoHandler_GetRAIS_FilterByAno(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			raisRecord("2024", []map[string]any{
				{"uf": "SP", "admissoes": float64(1000000)},
			}),
			raisRecord("2023", []map[string]any{
				{"uf": "SP", "admissoes": float64(900000)},
				{"uf": "RJ", "admissoes": float64(400000)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/rais?ano=2023", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items for year 2023, got %d", len(items))
	}
	periodo, _ := resp.Data["periodo"].(string)
	if periodo != "2023" {
		t.Errorf("expected periodo=2023, got %v", periodo)
	}
}

func TestEmpregoHandler_GetRAIS_FilterByAno_NotFound(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			raisRecord("2024", []map[string]any{
				{"uf": "SP", "admissoes": float64(1000000)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/rais?ano=2020", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEmpregoHandler_GetRAIS_NotFound(t *testing.T) {
	store := &empregoStore{records: nil}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/rais", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEmpregoHandler_GetRAIS_CombinedAnoAndUF(t *testing.T) {
	store := &empregoStore{
		records: []domain.SourceRecord{
			raisRecord("2024", []map[string]any{
				{"uf": "SP", "admissoes": float64(1000000)},
				{"uf": "RJ", "admissoes": float64(500000)},
			}),
			raisRecord("2023", []map[string]any{
				{"uf": "SP", "admissoes": float64(900000)},
				{"uf": "RJ", "admissoes": float64(400000)},
				{"uf": "MG", "admissoes": float64(300000)},
			}),
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/rais?ano=2023&uf=MG", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (MG in 2023), got %d", len(items))
	}
	total, _ := resp.Data["total"].(float64)
	if total != 1 {
		t.Errorf("expected total=1, got %v", total)
	}
}

// Test with in-memory typed items ([]map[string]any, not []any from JSONB).
// This verifies extractItems handles both type cases.
func TestEmpregoHandler_GetCAGED_InMemoryItems(t *testing.T) {
	// Directly build a record with []map[string]any items (not wrapped in []any).
	store := &empregoStore{
		records: []domain.SourceRecord{
			{
				Source:    "caged_emprego",
				RecordKey: "202501",
				Data: map[string]any{
					"periodo": "202501",
					"items": []map[string]any{
						{"uf": "SP", "admissoes": float64(100)},
						{"uf": "RJ", "admissoes": float64(50)},
					},
					"total": 2,
				},
				FetchedAt: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	h := handlers.NewEmpregoHandler(store)
	router := newEmpregoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/emprego/caged?uf=SP", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	items, ok := resp.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected data.items to be []any, got %T", resp.Data["items"])
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item for SP, got %d", len(items))
	}
}
