package handlers

import (
	"net/http"

	"github.com/databr/api/internal/domain"
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
	h.serveLatest(w, r, "ibge_ipca", "0.001")
}

// GetPIB handles GET /v1/economia/pib.
func (h *EconomiaHandler) GetPIB(w http.ResponseWriter, r *http.Request) {
	h.serveLatest(w, r, "ibge_pib", "0.001")
}

// GetFocus handles GET /v1/economia/focus.
// Retorna expectativas de mercado do Relatório Focus do BCB (expectativas anuais).
func (h *EconomiaHandler) GetFocus(w http.ResponseWriter, r *http.Request) {
	h.serveLatest(w, r, "bcb_focus", "0.001")
}

func (h *EconomiaHandler) serveLatest(w http.ResponseWriter, r *http.Request, source, price string) {
	records, err := h.store.FindLatest(r.Context(), source)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
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
		CostUSDC:  price,
		Data:      rec.Data,
	})
}
