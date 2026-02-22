package handlers

import (
	"net/http"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// MercadoHandler handles /v1/mercado/* requests (B3 stocks, CVM funds).
type MercadoHandler struct {
	store SourceStore
}

// NewMercadoHandler creates a Mercado handler.
func NewMercadoHandler(store SourceStore) *MercadoHandler {
	return &MercadoHandler{store: store}
}

// GetAcoes handles GET /v1/mercado/acoes/{ticker}.
// Returns the last available B3 closing price for the given ticker.
func (h *MercadoHandler) GetAcoes(w http.ResponseWriter, r *http.Request) {
	ticker := chi.URLParam(r, "ticker")
	rec, err := h.store.FindOne(r.Context(), "b3_cotacoes", ticker)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if rec == nil {
		jsonError(w, http.StatusNotFound, "No quote found for ticker "+ticker)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.002",
		Data:      rec.Data,
	})
}

// GetFundos handles GET /v1/mercado/fundos/{cnpj}.
// Returns CVM fund data for the given CNPJ.
func (h *MercadoHandler) GetFundos(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)
	rec, err := h.store.FindOne(r.Context(), "cvm_fundos", normalized)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if rec == nil {
		jsonError(w, http.StatusNotFound, "Fundo não encontrado: "+normalized)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.005",
		Data:      rec.Data,
	})
}
