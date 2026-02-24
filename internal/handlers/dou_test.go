package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/collectors/dou"
	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

type stubQDFetcher struct {
	records []domain.SourceRecord
	err     error
	cities  []domain.SourceRecord
	themes  []string
}

func (s *stubQDFetcher) Search(ctx context.Context, params dou.SearchParams) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func (s *stubQDFetcher) ListCities(ctx context.Context) ([]domain.SourceRecord, error) {
	return s.cities, s.err
}

func (s *stubQDFetcher) ListThemes(ctx context.Context) ([]string, error) {
	return s.themes, s.err
}

func (s *stubQDFetcher) SearchByTheme(ctx context.Context, theme string, params dou.SearchParams) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

func TestDOUHandler_GetBusca_OK(t *testing.T) {
	fetcher := &stubQDFetcher{
		records: []domain.SourceRecord{{
			Source:    "querido_diario",
			RecordKey: "contrato_0",
			Data:      map[string]any{"territory_name": "São Paulo", "date": "2026-02-01"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewDOUHandler(fetcher)
	r := chi.NewRouter()
	r.Get("/v1/dou/busca", h.GetBusca)

	req := httptest.NewRequest(http.MethodGet, "/v1/dou/busca?q=contrato", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", resp.Source)
	}
}

func TestDOUHandler_GetBusca_MissingQuery(t *testing.T) {
	h := handlers.NewDOUHandler(&stubQDFetcher{})
	r := chi.NewRouter()
	r.Get("/v1/dou/busca", h.GetBusca)

	req := httptest.NewRequest(http.MethodGet, "/v1/dou/busca", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDOUHandler_GetDiarios_OK(t *testing.T) {
	fetcher := &stubQDFetcher{
		records: []domain.SourceRecord{{
			Source:    "querido_diario",
			RecordKey: "licitacao_0",
			Data: map[string]any{
				"territory_name": "São Paulo",
				"date":           "2026-01-15",
				"territory_id":   "3550308",
			},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewDOUHandler(fetcher)
	r := chi.NewRouter()
	r.Get("/v1/diarios/busca", h.GetDiarios)

	req := httptest.NewRequest(http.MethodGet, "/v1/diarios/busca?q=licitacao&municipio_ibge=3550308&desde=2026-01-01", nil)
	req = x402pkg.InjectPrice(req, "0.007")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", resp.Source)
	}
	if resp.CostUSDC != "0.007" {
		t.Errorf("CostUSDC = %q, want 0.007", resp.CostUSDC)
	}
	if resp.Data == nil {
		t.Fatal("Data must not be nil")
	}
	if resp.Data["municipio_ibge"] != "3550308" {
		t.Errorf("Data[municipio_ibge] = %v, want 3550308", resp.Data["municipio_ibge"])
	}
}

func TestDOUHandler_GetDiarios_MissingQuery(t *testing.T) {
	h := handlers.NewDOUHandler(&stubQDFetcher{})
	r := chi.NewRouter()
	r.Get("/v1/diarios/busca", h.GetDiarios)

	req := httptest.NewRequest(http.MethodGet, "/v1/diarios/busca?municipio_ibge=3550308", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDOUHandler_GetDiarios_NoResults(t *testing.T) {
	h := handlers.NewDOUHandler(&stubQDFetcher{records: []domain.SourceRecord{}})
	r := chi.NewRouter()
	r.Get("/v1/diarios/busca", h.GetDiarios)

	req := httptest.NewRequest(http.MethodGet, "/v1/diarios/busca?q=termo+inexistente", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDOUHandler_GetMunicipios(t *testing.T) {
	fetcher := &stubQDFetcher{
		cities: []domain.SourceRecord{
			{
				Source:    "querido_diario",
				RecordKey: "3550308",
				Data:      map[string]any{"territory_id": "3550308", "territory_name": "São Paulo", "state_code": "SP"},
				FetchedAt: time.Now(),
			},
			{
				Source:    "querido_diario",
				RecordKey: "3304557",
				Data:      map[string]any{"territory_id": "3304557", "territory_name": "Rio de Janeiro", "state_code": "RJ"},
				FetchedAt: time.Now(),
			},
		},
	}
	h := handlers.NewDOUHandler(fetcher)
	r := chi.NewRouter()
	r.Get("/v1/diarios/municipios", h.GetMunicipios)

	req := httptest.NewRequest(http.MethodGet, "/v1/diarios/municipios", nil)
	req = x402pkg.InjectPrice(req, "0.003")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
	total, _ := resp.Data["total"].(float64)
	if int(total) != 2 {
		t.Errorf("total = %v, want 2", resp.Data["total"])
	}
}

func TestDOUHandler_GetTemas(t *testing.T) {
	fetcher := &stubQDFetcher{
		themes: []string{"Políticas Ambientais", "Tecnologias na Educação"},
	}
	h := handlers.NewDOUHandler(fetcher)
	r := chi.NewRouter()
	r.Get("/v1/diarios/temas", h.GetTemas)

	req := httptest.NewRequest(http.MethodGet, "/v1/diarios/temas", nil)
	req = x402pkg.InjectPrice(req, "0.003")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", resp.Source)
	}
	themes, ok := resp.Data["temas"]
	if !ok {
		t.Fatal("expected 'temas' key in response data")
	}
	themeList, ok := themes.([]any)
	if !ok {
		t.Fatalf("temas is %T, want []any", themes)
	}
	if len(themeList) != 2 {
		t.Errorf("len(temas) = %d, want 2", len(themeList))
	}
}

func TestDOUHandler_GetTema(t *testing.T) {
	fetcher := &stubQDFetcher{
		records: []domain.SourceRecord{{
			Source:    "querido_diario",
			RecordKey: "theme_Políticas Ambientais_0",
			Data:      map[string]any{"territory_name": "Campo Grande", "state_code": "MS", "date": "2023-05-16"},
			FetchedAt: time.Now(),
		}},
	}
	h := handlers.NewDOUHandler(fetcher)
	r := chi.NewRouter()
	r.Get("/v1/diarios/tema/{tema}", h.GetTema)

	req := httptest.NewRequest(http.MethodGet, "/v1/diarios/tema/Pol%C3%ADticas%20Ambientais", nil)
	req = x402pkg.InjectPrice(req, "0.005")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "querido_diario" {
		t.Errorf("Source = %q, want querido_diario", resp.Source)
	}
	if resp.CostUSDC != "0.005" {
		t.Errorf("CostUSDC = %q, want 0.005", resp.CostUSDC)
	}
	if resp.Data["tema"] != "Políticas Ambientais" {
		t.Errorf("Data[tema] = %v, want Políticas Ambientais", resp.Data["tema"])
	}
}
