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
)

// RREOFetcher fetches RREO data from Tesouro SICONFI on-demand.
type RREOFetcher interface {
	FetchRREO(ctx context.Context, uf string, ano, periodo int) ([]domain.SourceRecord, error)
}

// TesouroHandler handles /v1/tesouro/* endpoints.
type TesouroHandler struct {
	fetcher    RREOFetcher
	httpClient *http.Client
}

// NewTesouroHandler creates a Tesouro handler.
func NewTesouroHandler(fetcher RREOFetcher) *TesouroHandler {
	return &TesouroHandler{
		fetcher:    fetcher,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// NewTesouroHandlerWithClient creates a Tesouro handler with a custom HTTP client (useful for testing).
func NewTesouroHandlerWithClient(fetcher RREOFetcher, client *http.Client) *TesouroHandler {
	return &TesouroHandler{fetcher: fetcher, httpClient: client}
}

// GetRREO handles GET /v1/tesouro/rreo?uf=SP&ano=2024&periodo=1
// Returns the RREO (Resumo da Execução Orçamentária) for a Brazilian state.
func (h *TesouroHandler) GetRREO(w http.ResponseWriter, r *http.Request) {
	uf := r.URL.Query().Get("uf")
	if uf == "" {
		jsonError(w, http.StatusBadRequest, "query param 'uf' is required (ex: SP, RJ, BA)")
		return
	}
	if !isValidUF(uf) {
		jsonError(w, http.StatusBadRequest, "UF inválida: "+uf)
		return
	}

	ano := time.Now().Year()
	if s := r.URL.Query().Get("ano"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			ano = n
		}
	}
	periodo := 1
	if s := r.URL.Query().Get("periodo"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			periodo = n
		}
	}

	records, err := h.fetcher.FetchRREO(r.Context(), uf, ano, periodo)
	if err != nil {
		gatewayError(w, "tesouro", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No RREO data found for the given parameters")
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    "tesouro_siconfi",
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// siconfiItems calls the SICONFI endpoint and returns the "items" array.
func (h *TesouroHandler) siconfiItems(ctx context.Context, endpoint string) ([]any, error) {
	url := "https://apidatalake.tesouro.gov.br/ords/siconfi/tt//" + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		slog.Warn("SICONFI upstream error", "status", resp.StatusCode)
		return nil, fmt.Errorf("upstream service temporarily unavailable")
	}
	var envelope struct {
		Items []any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode SICONFI response: %w", err)
	}
	return envelope.Items, nil
}

// GetEntes handles GET /v1/tesouro/entes.
// Returns list of Brazilian municipalities and states from SICONFI.
// Optional query params: pagina (default 1), n (items per page, default 50, max 200).
func (h *TesouroHandler) GetEntes(w http.ResponseWriter, r *http.Request) {
	pagina := 1
	if raw := r.URL.Query().Get("pagina"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			pagina = v
		}
	}
	n := 50
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 200 {
			n = v
		}
	}

	endpoint := fmt.Sprintf("entes?pagina=%d&itensPorPagina=%d", pagina, n)
	items, err := h.siconfiItems(r.Context(), endpoint)
	if err != nil {
		gatewayError(w, "tesouro", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "siconfi_entes",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"entes": items, "total": len(items), "pagina": pagina},
	})
}

// GetRGF handles GET /v1/tesouro/rgf.
// Returns RGF (Relatório de Gestão Fiscal) data from SICONFI.
// Query params: uf (required), ano (default current year), esfera (default E for estados).
func (h *TesouroHandler) GetRGF(w http.ResponseWriter, r *http.Request) {
	uf := r.URL.Query().Get("uf")
	if uf == "" {
		jsonError(w, http.StatusBadRequest, "query param 'uf' is required (ex: SP, RJ, BA)")
		return
	}
	if !isValidUF(uf) {
		jsonError(w, http.StatusBadRequest, "UF inválida: "+uf)
		return
	}
	ano := time.Now().Year()
	if raw := r.URL.Query().Get("ano"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			ano = v
		}
	}
	esfera := r.URL.Query().Get("esfera")
	if esfera == "" {
		esfera = "E"
	}

	endpoint := fmt.Sprintf(
		"rgf?an_exercicio=%d&in_periodicidade=S&nr_periodo=1&co_tipo_demonstrativo=RGF&co_esfera=%s&no_uf=%s",
		ano, esfera, uf,
	)
	items, err := h.siconfiItems(r.Context(), endpoint)
	if err != nil {
		gatewayError(w, "tesouro", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "siconfi_rgf",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"rgf": items, "total": len(items), "uf": uf, "ano": ano},
	})
}

// GetDCA handles GET /v1/tesouro/dca.
// Returns DCA (Declaração de Contas Anuais) data from SICONFI.
// Optional query params: ano (default current year - 1), esfera (default M for municípios).
func (h *TesouroHandler) GetDCA(w http.ResponseWriter, r *http.Request) {
	ano := time.Now().Year() - 1 // DCA is typically for the prior year
	if raw := r.URL.Query().Get("ano"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			ano = v
		}
	}
	esfera := r.URL.Query().Get("esfera")
	if esfera == "" {
		esfera = "M"
	}

	endpoint := fmt.Sprintf("dca?an_exercicio=%d&co_esfera=%s", ano, esfera)
	items, err := h.siconfiItems(r.Context(), endpoint)
	if err != nil {
		gatewayError(w, "tesouro", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "siconfi_dca",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"dca": items, "total": len(items), "ano": ano, "esfera": esfera},
	})
}
