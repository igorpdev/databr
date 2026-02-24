package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/collectors/tributario"
	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// --- stubs ---

type stubIBPTFetcher struct {
	records []domain.SourceRecord
	err     error
}

func (s *stubIBPTFetcher) FetchByNCM(_ context.Context, codigo, uf string) ([]domain.SourceRecord, error) {
	return s.records, s.err
}

type stubICMSQuerier struct{}

func (s *stubICMSQuerier) GetInternalRate(uf string) (*tributario.ICMSRate, error) {
	p := tributario.NewICMSProvider()
	return p.GetInternalRate(uf)
}

func (s *stubICMSQuerier) GetInterstateRate(origem, destino string) (*tributario.InterstateRate, error) {
	p := tributario.NewICMSProvider()
	return p.GetInterstateRate(origem, destino)
}

func (s *stubICMSQuerier) GetAllRates() []tributario.ICMSRate {
	p := tributario.NewICMSProvider()
	return p.GetAllRates()
}

// --- helper ---

func newTributarioRouter(h *handlers.TributarioHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/tributario/ncm/{codigo}", h.GetNCMTributos)
	r.Get("/v1/tributario/icms/{uf}", h.GetICMS)
	r.Get("/v1/tributario/icms", h.GetICMS)
	return r
}

func doRequest(router http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req = x402pkg.InjectPrice(req, "0.003")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// --- NCM tests ---

func TestTributarioHandler_GetNCMTributos_OK(t *testing.T) {
	fetcher := &stubIBPTFetcher{
		records: []domain.SourceRecord{{
			Source:    "ibpt_tributos",
			RecordKey: "22030000_sp",
			Data: map[string]any{
				"codigo":    "22030000",
				"descricao": "Cervejas de malte",
				"tipo":      "ncm",
				"uf":        "SP",
				"aliquotas": map[string]any{
					"federal_nacional": 13.91,
					"estadual":         22.0,
				},
			},
			FetchedAt: time.Now().UTC(),
		}},
	}
	h := handlers.NewTributarioHandler(fetcher, &stubICMSQuerier{})
	router := newTributarioRouter(h)

	rr := doRequest(router, "GET", "/v1/tributario/ncm/22030000?uf=SP")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Source != "ibpt_tributos" {
		t.Errorf("Source = %q, want ibpt_tributos", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
}

func TestTributarioHandler_GetNCMTributos_MissingUF(t *testing.T) {
	h := handlers.NewTributarioHandler(&stubIBPTFetcher{}, &stubICMSQuerier{})
	router := newTributarioRouter(h)

	rr := doRequest(router, "GET", "/v1/tributario/ncm/22030000")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestTributarioHandler_GetNCMTributos_InvalidUF(t *testing.T) {
	h := handlers.NewTributarioHandler(&stubIBPTFetcher{}, &stubICMSQuerier{})
	router := newTributarioRouter(h)

	rr := doRequest(router, "GET", "/v1/tributario/ncm/22030000?uf=ZZ")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestTributarioHandler_GetNCMTributos_NotFound(t *testing.T) {
	fetcher := &stubIBPTFetcher{
		err: fmt.Errorf("ibpt: NCM 99999999 not found in SP"),
	}
	h := handlers.NewTributarioHandler(fetcher, &stubICMSQuerier{})
	router := newTributarioRouter(h)

	rr := doRequest(router, "GET", "/v1/tributario/ncm/99999999?uf=SP")
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// --- ICMS tests ---

func TestTributarioHandler_GetICMS_Internal(t *testing.T) {
	h := handlers.NewTributarioHandler(&stubIBPTFetcher{}, &stubICMSQuerier{})
	router := newTributarioRouter(h)

	rr := doRequest(router, "GET", "/v1/tributario/icms/SP")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Source != "icms_aliquotas" {
		t.Errorf("Source = %q, want icms_aliquotas", resp.Source)
	}
	if resp.Data["aliquota_interna"] != 18.0 {
		t.Errorf("aliquota_interna = %v, want 18.0", resp.Data["aliquota_interna"])
	}
}

func TestTributarioHandler_GetICMS_Interstate(t *testing.T) {
	h := handlers.NewTributarioHandler(&stubIBPTFetcher{}, &stubICMSQuerier{})
	router := newTributarioRouter(h)

	rr := doRequest(router, "GET", "/v1/tributario/icms?origem=SP&destino=MA")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Data["aliquota_interestadual"] != 7.0 {
		t.Errorf("aliquota_interestadual = %v, want 7.0", resp.Data["aliquota_interestadual"])
	}
	if resp.Data["difal"] != 16.0 {
		t.Errorf("difal = %v, want 16.0", resp.Data["difal"])
	}
}

func TestTributarioHandler_GetICMS_AllStates(t *testing.T) {
	h := handlers.NewTributarioHandler(&stubIBPTFetcher{}, &stubICMSQuerier{})
	router := newTributarioRouter(h)

	rr := doRequest(router, "GET", "/v1/tributario/icms")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp domain.APIResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Data["total"] != 27.0 { // JSON numbers are float64
		t.Errorf("total = %v, want 27", resp.Data["total"])
	}
}

func TestTributarioHandler_GetICMS_InvalidUF(t *testing.T) {
	h := handlers.NewTributarioHandler(&stubIBPTFetcher{}, &stubICMSQuerier{})
	router := newTributarioRouter(h)

	rr := doRequest(router, "GET", "/v1/tributario/icms/ZZ")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}
