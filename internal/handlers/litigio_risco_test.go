package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

func newLitigioRiscoRouter(h *handlers.LitigioRiscoHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/litigio/{cnpj}/risco", h.GetLitigioRisco)
	return r
}

func TestLitigioRisco_OK_NoProcesses(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data:      map[string]any{"cnpj": "11222333000181", "razao_social": "EMPRESA TESTE LTDA"},
			FetchedAt: time.Now(),
		}},
	}
	judicialSearcher := &ddJudicialSearcher{records: nil}
	store := &ddStore{records: nil}

	h := handlers.NewLitigioRiscoHandler(judicialSearcher, cnpjFetcher, store)
	router := newLitigioRiscoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/litigio/11222333000181/risco", nil)
	req = x402pkg.InjectPrice(req, "0.030")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "litigio_risco" {
		t.Errorf("Source = %q, want litigio_risco", resp.Source)
	}
	if resp.CostUSDC != "0.030" {
		t.Errorf("CostUSDC = %q, want 0.030", resp.CostUSDC)
	}

	// With no processes the risk score should be 0.
	riscoLitigio, ok := resp.Data["risco_litigio"].(map[string]any)
	if !ok {
		t.Fatalf("risco_litigio not found in response data")
	}
	score, _ := riscoLitigio["score"].(float64)
	if score != 0 {
		t.Errorf("risco_litigio.score = %v, want 0", score)
	}
	nivel, _ := riscoLitigio["nivel"].(string)
	if nivel != "baixo" {
		t.Errorf("risco_litigio.nivel = %q, want baixo", nivel)
	}

	// Verify processos_resumo.total == 0.
	resumo, ok := resp.Data["processos_resumo"].(map[string]any)
	if !ok {
		t.Fatalf("processos_resumo not found")
	}
	total, _ := resumo["total"].(float64)
	if total != 0 {
		t.Errorf("processos_resumo.total = %v, want 0", total)
	}
}

func TestLitigioRisco_WithProcesses(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data:      map[string]any{"cnpj": "11222333000181", "razao_social": "EMPRESA TESTE LTDA"},
			FetchedAt: time.Now(),
		}},
	}

	// 5 civil processes, all recent (within 12 months), CNPJ as defendant.
	processes := make([]domain.SourceRecord, 5)
	recentDate := time.Now().AddDate(0, -3, 0).Format("2006-01-02")
	for i := range processes {
		processes[i] = domain.SourceRecord{
			Source: "datajud_cnj",
			Data: map[string]any{
				"classeProcessual": "Acao Civel",
				"dataAjuizamento":  recentDate,
				"poloPassivo": []any{
					map[string]any{"documento": "11222333000181"},
				},
			},
		}
	}

	judicialSearcher := &ddJudicialSearcher{records: processes}
	store := &ddStore{records: nil}

	h := handlers.NewLitigioRiscoHandler(judicialSearcher, cnpjFetcher, store)
	router := newLitigioRiscoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/litigio/11222333000181/risco", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Check processos_resumo.
	resumo, ok := resp.Data["processos_resumo"].(map[string]any)
	if !ok {
		t.Fatalf("processos_resumo not found")
	}
	total, _ := resumo["total"].(float64)
	if total != 5 {
		t.Errorf("processos_resumo.total = %v, want 5", total)
	}
	comoReu, _ := resumo["como_reu"].(float64)
	if comoReu != 5 {
		t.Errorf("processos_resumo.como_reu = %v, want 5", comoReu)
	}

	// Check risk score > 0.
	riscoLitigio, ok := resp.Data["risco_litigio"].(map[string]any)
	if !ok {
		t.Fatalf("risco_litigio not found")
	}
	score, _ := riscoLitigio["score"].(float64)
	if score <= 0 {
		t.Errorf("risco_litigio.score = %v, want > 0", score)
	}

	// 5 processes: base=10, defendant ratio 100%=+20, prior=0 recent=5 so +15 = 45.
	// Civel distribution adds nothing extra. Expected = 45.
	if score != 45 {
		t.Errorf("risco_litigio.score = %v, want 45", score)
	}

	// Check distribution has civel > 0.
	dist, ok := resp.Data["distribuicao_tipos"].(map[string]any)
	if !ok {
		t.Fatalf("distribuicao_tipos not found")
	}
	civel, _ := dist["civel"].(float64)
	if civel != 5 {
		t.Errorf("distribuicao_tipos.civel = %v, want 5", civel)
	}
}

func TestLitigioRisco_InvalidCNPJ(t *testing.T) {
	h := handlers.NewLitigioRiscoHandler(
		&ddJudicialSearcher{},
		&ddCNPJFetcher{},
		&ddStore{},
	)
	router := newLitigioRiscoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/litigio/123/risco", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLitigioRisco_JudicialError(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data:      map[string]any{"cnpj": "11222333000181"},
			FetchedAt: time.Now(),
		}},
	}
	judicialSearcher := &ddJudicialSearcher{err: errors.New("datajud unavailable")}
	store := &ddStore{records: nil}

	h := handlers.NewLitigioRiscoHandler(judicialSearcher, cnpjFetcher, store)
	router := newLitigioRiscoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/litigio/11222333000181/risco", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLitigioRisco_CriminalProcesses(t *testing.T) {
	cnpjFetcher := &ddCNPJFetcher{
		records: []domain.SourceRecord{{
			Source:    "cnpj",
			RecordKey: "11222333000181",
			Data:      map[string]any{"cnpj": "11222333000181", "razao_social": "EMPRESA TESTE LTDA"},
			FetchedAt: time.Now(),
		}},
	}

	// 2 criminal processes.
	recentDate := time.Now().AddDate(0, -2, 0).Format("2006-01-02")
	processes := []domain.SourceRecord{
		{
			Source: "datajud_cnj",
			Data: map[string]any{
				"classeProcessual": "Acao Penal criminal",
				"dataAjuizamento":  recentDate,
			},
		},
		{
			Source: "datajud_cnj",
			Data: map[string]any{
				"classeProcessual": "criminal especial",
				"dataAjuizamento":  recentDate,
			},
		},
	}

	judicialSearcher := &ddJudicialSearcher{records: processes}
	store := &ddStore{records: nil}

	h := handlers.NewLitigioRiscoHandler(judicialSearcher, cnpjFetcher, store)
	router := newLitigioRiscoRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/litigio/11222333000181/risco", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	dist, ok := resp.Data["distribuicao_tipos"].(map[string]any)
	if !ok {
		t.Fatalf("distribuicao_tipos not found")
	}
	criminal, _ := dist["criminal"].(float64)
	if criminal <= 0 {
		t.Errorf("distribuicao_tipos.criminal = %v, want > 0", criminal)
	}
	if criminal != 2 {
		t.Errorf("distribuicao_tipos.criminal = %v, want 2", criminal)
	}

	// Risk score should include +15 for criminal processes.
	riscoLitigio, ok := resp.Data["risco_litigio"].(map[string]any)
	if !ok {
		t.Fatalf("risco_litigio not found")
	}
	score, _ := riscoLitigio["score"].(float64)
	// 2 processes: base=5, prior=0 recent=2 so +15 (new litigation), criminal=+15 => 35
	if score != 35 {
		t.Errorf("risco_litigio.score = %v, want 35", score)
	}
}
