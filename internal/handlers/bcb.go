package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
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
		gatewayError(w, "bcb", err)
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
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// GetCambio handles GET /v1/bcb/cambio/{moeda}.
// Returns the most recent available PTAX rate for the given currency.
func (h *BCBHandler) GetCambio(w http.ResponseWriter, r *http.Request) {
	moeda := chi.URLParam(r, "moeda")

	records, err := h.store.FindLatest(r.Context(), "bcb_ptax")
	if err != nil {
		gatewayError(w, "bcb", err)
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
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      match.Data,
	})
}

// GetPIX handles GET /v1/bcb/pix/estatisticas.
func (h *BCBHandler) GetPIX(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_pix")
	if err != nil {
		gatewayError(w, "bcb", err)
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
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// GetCredito handles GET /v1/bcb/credito.
func (h *BCBHandler) GetCredito(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_credito")
	if err != nil {
		gatewayError(w, "bcb", err)
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
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// GetReservas handles GET /v1/bcb/reservas.
func (h *BCBHandler) GetReservas(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_reservas")
	if err != nil {
		gatewayError(w, "bcb", err)
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
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// GetTaxasCredito handles GET /v1/bcb/taxas-credito.
// Returns the latest credit market interest rates from BCB OLINDA (TaxaJuros service).
func (h *BCBHandler) GetTaxasCredito(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_taxas_credito")
	if err != nil {
		gatewayError(w, "bcb", err)
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
		CostUSDC:  x402pkg.PriceFromRequest(r),
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
		internalError(w, "bcb", err)
		return
	}

	upResp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "bcb", err)
		return
	}
	defer upResp.Body.Close()

	if upResp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(upResp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("BCB SGS", upResp.StatusCode, body))
		return
	}

	var valores []map[string]any
	if err := json.NewDecoder(upResp.Body).Decode(&valores); err != nil {
		gatewayError(w, "bcb", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "bcb_sgs",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"serie":  nomeSerie,
			"codigo": code,
			"n":      n,
			"valores": valores,
		},
	})
}

// GetCapitais handles GET /v1/bcb/capitais.
// Returns recent registrations of foreign direct investment (IED) from BCB OLINDA.
// Optional query param: n (number of results, default 20, max 100).
func (h *BCBHandler) GetCapitais(w http.ResponseWriter, r *http.Request) {
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	upURL := fmt.Sprintf(
		"https://olinda.bcb.gov.br/olinda/servico/RDE_Publicacao/versao/v1/odata/RegistrosIED?$top=%d&$format=json",
		n,
	)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "bcb", err)
		return
	}

	upResp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "bcb", err)
		return
	}
	defer upResp.Body.Close()

	if upResp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(upResp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("BCB OLINDA RDE", upResp.StatusCode, body))
		return
	}

	var envelope struct {
		Value []map[string]any `json:"value"`
	}
	if err := json.NewDecoder(upResp.Body).Decode(&envelope); err != nil {
		gatewayError(w, "bcb", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "bcb_rde",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"registros": envelope.Value,
			"total":     len(envelope.Value),
		},
	})
}

// GetSML handles GET /v1/bcb/sml.
// Returns the latest SML exchange rates between Brazil and Paraguay, Uruguay, and Argentina.
// Optional query param: pais (paraguai|uruguai|argentina|all, default "all").
func (h *BCBHandler) GetSML(w http.ResponseWriter, r *http.Request) {
	pais := r.URL.Query().Get("pais")
	if pais == "" {
		pais = "all"
	}

	// Map friendly name to OLINDA entity name suffix (case-sensitive).
	paisMap := map[string]string{
		"paraguai":  "Paraguai",
		"uruguai":   "Uruguai",
		"argentina": "Argentina",
	}

	type smlResult struct {
		Pais  string           `json:"pais"`
		Dados []map[string]any `json:"dados"`
	}

	fetch := func(ctx context.Context, suffix string) ([]map[string]any, error) {
		upURL := fmt.Sprintf(
			"https://olinda.bcb.gov.br/olinda/servico/SML/versao/v1/odata/CotacaoTaxaSMLBrasil%s?$top=5&$format=json",
			suffix,
		)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, upURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := h.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("BCB SML returned %d for %s", resp.StatusCode, suffix)
		}
		var env struct {
			Value []map[string]any `json:"value"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
			return nil, err
		}
		return env.Value, nil
	}

	if pais != "all" {
		suffix, ok := paisMap[pais]
		if !ok {
			jsonError(w, http.StatusBadRequest, "pais inválido — use paraguai, uruguai, argentina ou all")
			return
		}
		dados, err := fetch(r.Context(), suffix)
		if err != nil {
			gatewayError(w, "bcb", err)
			return
		}
		respond(w, r, domain.APIResponse{
			Source:   "bcb_sml",
			CostUSDC: x402pkg.PriceFromRequest(r),
			Data:     map[string]any{"pais": pais, "cotacoes": dados, "total": len(dados)},
		})
		return
	}

	// Fetch all 3 countries.
	var resultados []smlResult
	for name, suffix := range paisMap {
		dados, err := fetch(r.Context(), suffix)
		if err != nil {
			continue // skip unavailable
		}
		resultados = append(resultados, smlResult{Pais: name, Dados: dados})
	}

	respond(w, r, domain.APIResponse{
		Source:   "bcb_sml",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"paises": resultados},
	})
}

// GetIFData handles GET /v1/bcb/ifdata.
// Returns financial institution data from BCB OLINDA IFDATA service.
// Uses the IfDataCadastro function import (requires AnoMes=YYYYMM parameter).
// Optional query params:
//   - n: number of results to return (default 10, max 100)
//   - data: reference date in format YYYYMM (e.g. "202412"); defaults to last completed quarter
func (h *BCBHandler) GetIFData(w http.ResponseWriter, r *http.Request) {
	n := 10
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	// Use provided date or calculate a safely-published quarter (YYYYMM).
	// BCB IFDATA publishes quarterly data with ~3-4 month delay; going back 6 months
	// guarantees the quarter has been published.
	anoMes := r.URL.Query().Get("data")
	if anoMes == "" {
		t := time.Now().UTC().AddDate(0, -6, 0)
		month := int(t.Month())
		year := t.Year()
		// Snap forward to the end of that month's quarter (Q1→3, Q2→6, Q3→9, Q4→12).
		quarterEnd := ((month-1)/3 + 1) * 3
		anoMes = fmt.Sprintf("%d%02d", year, quarterEnd)
	}

	// OLINDA IFDATA uses a FunctionImport (not EntitySet) — the correct form is
	// IfDataCadastro(AnoMes=YYYYMM), not _IfDataCadastro.
	upURL := fmt.Sprintf(
		"https://olinda.bcb.gov.br/olinda/servico/IFDATA/versao/v1/odata/IfDataCadastro(AnoMes=%s)?$top=%d&$format=json&$orderby=NomeInstituicao",
		anoMes, n,
	)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "bcb", err)
		return
	}

	upResp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "bcb", err)
		return
	}
	defer upResp.Body.Close()

	if upResp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(upResp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("BCB OLINDA IFDATA", upResp.StatusCode, body))
		return
	}

	var envelope struct {
		Value []map[string]any `json:"value"`
	}
	if err := json.NewDecoder(upResp.Body).Decode(&envelope); err != nil {
		gatewayError(w, "bcb", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "bcb_ifdata",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"instituicoes": envelope.Value,
			"total":        len(envelope.Value),
		},
	})
}

// GetBaseMonetaria handles GET /v1/bcb/base-monetaria.
// Returns the last n observations of Brazilian monetary aggregates (M0, M2) from BCB SGS.
// Optional query param: n (default 12, max 100).
// Series used:
//   - 27790: M0 – Base monetária restrita
//   - 27791: M2 – M1 + depósitos especiais remunerados + depósitos de poupança + títulos emitidos por IF
func (h *BCBHandler) GetBaseMonetaria(w http.ResponseWriter, r *http.Request) {
	n := 12
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	type serieResult struct {
		nome   string
		series []map[string]any
		err    error
	}

	fetchSGS := func(code int) ([]map[string]any, error) {
		url := fmt.Sprintf("https://api.bcb.gov.br/dados/serie/bcdata.sgs.%d/dados/ultimos/%d?formato=json", code, n)
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := h.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			_, _ = limitedReadAll(resp.Body) // drain body
			slog.Warn("BCB SGS upstream error", "series", code, "status", resp.StatusCode)
			return nil, fmt.Errorf("upstream service temporarily unavailable")
		}
		var valores []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&valores); err != nil {
			return nil, fmt.Errorf("failed to decode BCB SGS %d: %w", code, err)
		}
		return valores, nil
	}

	m0, errM0 := fetchSGS(27790)
	if errM0 != nil {
		gatewayError(w, "bcb", errM0)
		return
	}

	m2, errM2 := fetchSGS(27791)
	if errM2 != nil {
		gatewayError(w, "bcb", errM2)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "bcb_base_monetaria",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"m0":          m0,
			"m2":          m2,
			"n":           n,
			"descricao_m0": "M0 - Base monetária restrita (R$ milhares)",
			"descricao_m2": "M2 - M1 + depósitos de poupança + títulos emitidos por IF (R$ milhares)",
		},
	})
}

// jsonError and respond are defined in helpers.go
