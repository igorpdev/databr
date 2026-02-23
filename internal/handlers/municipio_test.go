package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// mockIBGEServer returns a test server that maps IBGE codes to names.
func mockIBGEServer(mapping map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		code := parts[len(parts)-1]
		if name, ok := mapping[code]; ok {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"nome":"%s"}`, name)
			return
		}
		http.NotFound(w, r)
	}))
}

// municipioStore implements handlers.SourceStore for municipality tests.
type municipioStore struct {
	filteredRecords map[string][]domain.SourceRecord
	err             error
}

func (s *municipioStore) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	return nil, nil
}

func (s *municipioStore) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	return nil, nil
}

func (s *municipioStore) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
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

func newMunicipioRouter(h *handlers.MunicipioHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/municipios/{codigo}/perfil", h.GetMunicipioPerfil)
	return r
}

func TestMunicipio_OK_AllData(t *testing.T) {
	// Mock IBGE API to return "Sao Paulo" for code "3550308"
	ibgeSrv := mockIBGEServer(map[string]string{"3550308": "Sao Paulo"})
	defer ibgeSrv.Close()
	handlers.SetIBGEBaseURL(ibgeSrv.URL)
	defer handlers.SetIBGEBaseURL("")

	store := &municipioStore{
		filteredRecords: map[string][]domain.SourceRecord{
			"ibge_populacao": {{
				Source: "ibge_populacao",
				Data:   map[string]any{"codigo": "3550308", "populacao": float64(12400000), "nome": "Sao Paulo"},
			}},
			"inpe_deter": {
				{Source: "inpe_deter", Data: map[string]any{"municipio": "Sao Paulo", "area_km2": 0.1}},
				{Source: "inpe_deter", Data: map[string]any{"municipio": "Sao Paulo", "area_km2": 0.2}},
			},
			"pncp_licitacoes": {{
				Source: "pncp_licitacoes",
				Data:   map[string]any{"municipio": "3550308", "objeto": "Construcao"},
			}},
		},
	}

	h := handlers.NewMunicipioHandler(store)
	router := newMunicipioRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/municipios/3550308/perfil", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "municipio_perfil" {
		t.Errorf("Source = %q, want municipio_perfil", resp.Source)
	}
	if resp.CostUSDC != "0.030" {
		t.Errorf("CostUSDC = %q, want 0.030", resp.CostUSDC)
	}
	if resp.Data["codigo"] != "3550308" {
		t.Errorf("codigo = %v, want 3550308", resp.Data["codigo"])
	}

	deterCount, _ := resp.Data["deter_alert_count"].(float64)
	if deterCount != 2 {
		t.Errorf("deter_alert_count = %v, want 2", deterCount)
	}
	licitacoesCount, _ := resp.Data["licitacoes_count"].(float64)
	if licitacoesCount != 1 {
		t.Errorf("licitacoes_count = %v, want 1", licitacoesCount)
	}
	// populacao should be a map with data.
	populacao, _ := resp.Data["populacao"].(map[string]any)
	if populacao == nil {
		t.Error("expected populacao to be a non-nil map")
	}
}

func TestMunicipio_NoData(t *testing.T) {
	store := &municipioStore{
		filteredRecords: map[string][]domain.SourceRecord{
			"ibge_populacao":  {},
			"inpe_deter":      {},
			"pncp_licitacoes": {},
		},
	}

	h := handlers.NewMunicipioHandler(store)
	router := newMunicipioRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/municipios/9999999/perfil", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even with no data, got %d", rec.Code)
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	deterCount, _ := resp.Data["deter_alert_count"].(float64)
	if deterCount != 0 {
		t.Errorf("deter_alert_count = %v, want 0", deterCount)
	}
	licitacoesCount, _ := resp.Data["licitacoes_count"].(float64)
	if licitacoesCount != 0 {
		t.Errorf("licitacoes_count = %v, want 0", licitacoesCount)
	}
}

func TestMunicipio_PopulacaoOnly(t *testing.T) {
	store := &municipioStore{
		filteredRecords: map[string][]domain.SourceRecord{
			"ibge_populacao": {{
				Source: "ibge_populacao",
				Data:   map[string]any{"codigo": "5300108", "populacao": float64(3000000), "nome": "Brasilia"},
			}},
			"inpe_deter":      {},
			"pncp_licitacoes": {},
		},
	}

	h := handlers.NewMunicipioHandler(store)
	router := newMunicipioRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/municipios/5300108/perfil", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp domain.APIResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	populacao, _ := resp.Data["populacao"].(map[string]any)
	if populacao == nil {
		t.Error("expected populacao to have data")
	}
	nome, _ := populacao["nome"].(string)
	if nome != "Brasilia" {
		t.Errorf("populacao.nome = %q, want Brasilia", nome)
	}
}
