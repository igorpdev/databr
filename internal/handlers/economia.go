package handlers

import (
	"net/http"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
)

// EconomiaHandler handles requests for /v1/economia/*.
type EconomiaHandler struct {
	store SourceStore
}

// NewEconomiaHandler creates an Economia handler.
func NewEconomiaHandler(store SourceStore) *EconomiaHandler {
	return &EconomiaHandler{store: store}
}

// GetIPCA handles GET /v1/economia/ipca.
func (h *EconomiaHandler) GetIPCA(w http.ResponseWriter, r *http.Request) {
	h.serveLatest(w, r, "ibge_ipca")
}

// GetPIB handles GET /v1/economia/pib.
func (h *EconomiaHandler) GetPIB(w http.ResponseWriter, r *http.Request) {
	h.serveLatest(w, r, "ibge_pib")
}

// GetFocus handles GET /v1/economia/focus.
// Retorna expectativas de mercado do Relatório Focus do BCB (expectativas anuais).
func (h *EconomiaHandler) GetFocus(w http.ResponseWriter, r *http.Request) {
	h.serveLatest(w, r, "bcb_focus")
}

func (h *EconomiaHandler) serveLatest(w http.ResponseWriter, r *http.Request, source string) {
	records, err := h.store.FindLatest(r.Context(), source)
	if err != nil {
		gatewayError(w, "economia", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, source+" data not yet available")
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
