package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// SourceStore is the minimal interface needed by BCB and Economia handlers.
type SourceStore interface {
	FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error)
	FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error)
	// FindLatestFiltered returns records for the given source where the JSONB
	// data field at jsonbKey contains jsonbValue (case-insensitive substring).
	// Useful for large datasets like ANEEL where in-memory filtering is impractical.
	FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error)
}

// sgsIndicadores maps friendly names to BCB SGS series codes and descriptions.
var sgsIndicadores = map[string]struct {
	code int
	nome string
}{
	"cdi":         {12, "CDI - Certificado de Depósito Interbancário"},
	"selic":       {11, "SELIC - Taxa básica acumulada no período"},
	"selic-meta":  {4392, "SELIC meta fixada pelo COPOM"},
	"igpm":        {189, "IGP-M - Índice Geral de Preços ao Mercado"},
	"dolar":       {1, "Dólar americano (compra) - cotação diária"},
	"desemprego":  {7326, "Taxa de desemprego - Pesquisa Mensal de Emprego"},
	"ipca-mensal": {433, "IPCA - variação mensal"},
	"inpc":        {188, "INPC - variação mensal"},
	"igp-di":      {190, "IGP-DI - variação mensal"},
	"poupanca":    {195, "Poupança - rendimento mensal"},
}

// BCBHandler handles requests for /v1/bcb/*.
type BCBHandler struct {
	store      SourceStore
	httpClient *http.Client
}

// NewBCBHandler creates a BCB handler.
func NewBCBHandler(store SourceStore) *BCBHandler {
	return &BCBHandler{
		store:      store,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// NewBCBHandlerWithClient creates a BCB handler with a custom HTTP client (useful for testing).
func NewBCBHandlerWithClient(store SourceStore, client *http.Client) *BCBHandler {
	return &BCBHandler{store: store, httpClient: client}
}

// GetSelic handles GET /v1/bcb/selic.
func (h *BCBHandler) GetSelic(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_selic")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "Selic data not yet available")
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetCambio handles GET /v1/bcb/cambio/{moeda}.
// Returns the most recent available PTAX rate for the given currency.
func (h *BCBHandler) GetCambio(w http.ResponseWriter, r *http.Request) {
	moeda := chi.URLParam(r, "moeda")

	records, err := h.store.FindLatest(r.Context(), "bcb_ptax")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Filter by currency: record key is "<MOEDA>_<DATE>"
	var match *domain.SourceRecord
	for i := range records {
		if m, ok := records[i].Data["moeda"].(string); ok && m == moeda {
			match = &records[i]
			break
		}
	}
	if match == nil {
		jsonError(w, http.StatusNotFound, "Exchange rate not found for "+moeda)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    match.Source,
		UpdatedAt: match.FetchedAt,
		CostUSDC:  "0.001",
		Data:      match.Data,
	})
}

// GetPIX handles GET /v1/bcb/pix/estatisticas.
func (h *BCBHandler) GetPIX(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_pix")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "PIX data not yet available")
		return
	}
	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetCredito handles GET /v1/bcb/credito.
func (h *BCBHandler) GetCredito(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_credito")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "Credit data not yet available")
		return
	}
	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetReservas handles GET /v1/bcb/reservas.
func (h *BCBHandler) GetReservas(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_reservas")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "Reserves data not yet available")
		return
	}
	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetTaxasCredito handles GET /v1/bcb/taxas-credito.
// Returns the latest credit market interest rates from BCB OLINDA (TaxaJuros service).
func (h *BCBHandler) GetTaxasCredito(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_taxas_credito")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "Taxas de crédito não disponíveis ainda")
		return
	}

	taxas := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		taxas = append(taxas, rec.Data)
	}

	respond(w, r, domain.APIResponse{
		Source:    "bcb_taxas_credito",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  "0.001",
		Data:      map[string]any{"taxas": taxas},
	})
}

// GetIndicadores handles GET /v1/bcb/indicadores/{serie}.
// serie can be a friendly name (cdi, igpm, dolar, etc.) or a numeric SGS code.
// Optional query param: n (number of values, default 12, max 100).
func (h *BCBHandler) GetIndicadores(w http.ResponseWriter, r *http.Request) {
	serie := chi.URLParam(r, "serie")

	// Resolve series code and name.
	var code int
	var nomeSerie string

	if ind, ok := sgsIndicadores[serie]; ok {
		code = ind.code
		nomeSerie = ind.nome
	} else {
		parsed, err := strconv.Atoi(serie)
		if err != nil || parsed <= 0 {
			jsonError(w, http.StatusBadRequest, "Série inválida. Use um nome (cdi, igpm, dolar, selic, selic-meta, desemprego, ipca-mensal, inpc, igp-di, poupanca) ou código numérico SGS.")
			return
		}
		code = parsed
		nomeSerie = fmt.Sprintf("BCB SGS Série %d", code)
	}

	// Number of values to return (default 12, max 100).
	n := 12
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 && parsed <= 100 {
			n = parsed
		}
	}

	url := fmt.Sprintf("https://api.bcb.gov.br/dados/serie/bcdata.sgs.%d/dados/ultimos/%d?formato=json", code, n)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to build request: "+err.Error())
		return
	}

	upResp, err := h.httpClient.Do(req)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "BCB SGS unavailable: "+err.Error())
		return
	}
	defer upResp.Body.Close()

	if upResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(upResp.Body)
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("BCB SGS returned %d: %s", upResp.StatusCode, string(body)))
		return
	}

	var valores []map[string]any
	if err := json.NewDecoder(upResp.Body).Decode(&valores); err != nil {
		jsonError(w, http.StatusBadGateway, "failed to decode BCB SGS response: "+err.Error())
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "bcb_sgs",
		CostUSDC: "0.001",
		Data: map[string]any{
			"serie":  nomeSerie,
			"codigo": code,
			"n":      n,
			"valores": valores,
		},
	})
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// respond writes the API response, applying ?format=context if requested.
func respond(w http.ResponseWriter, r *http.Request, resp domain.APIResponse) {
	if r.URL.Query().Get("format") == "context" {
		b, err := json.Marshal(resp.Data)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to serialize context")
			return
		}
		resp.Context = fmt.Sprintf("[%s] %s", resp.Source, string(b))
		resp.Data = nil
		// Add $0.001 using integer milliUSDC to avoid float rounding
		if f, err := strconv.ParseFloat(resp.CostUSDC, 64); err == nil {
			millis := int64(math.Round(f * 1000))
			resp.CostUSDC = fmt.Sprintf("%.3f", float64(millis+1)/1000.0)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
