package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/databr/api/internal/domain"
)

// RREOFetcher fetches RREO data from Tesouro SICONFI on-demand.
type RREOFetcher interface {
	FetchRREO(ctx context.Context, uf string, ano, periodo int) ([]domain.SourceRecord, error)
}

// TesouroHandler handles /v1/tesouro/* endpoints.
type TesouroHandler struct {
	fetcher RREOFetcher
}

// NewTesouroHandler creates a Tesouro handler.
func NewTesouroHandler(fetcher RREOFetcher) *TesouroHandler {
	return &TesouroHandler{fetcher: fetcher}
}

// GetRREO handles GET /v1/tesouro/rreo?uf=SP&ano=2024&periodo=1
// Returns the RREO (Resumo da Execução Orçamentária) for a Brazilian state.
func (h *TesouroHandler) GetRREO(w http.ResponseWriter, r *http.Request) {
	uf := r.URL.Query().Get("uf")
	if uf == "" {
		jsonError(w, http.StatusBadRequest, "query param 'uf' is required (ex: SP, RJ, BA)")
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
		jsonError(w, http.StatusBadGateway, err.Error())
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
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}
