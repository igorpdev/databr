package handlers

import (
	"net/http"

	"github.com/databr/api/internal/domain"
)

// AmbientalHandler handles requests for /v1/ambiental/*.
// It uses the SourceStore interface already defined in bcb.go.
type AmbientalHandler struct {
	store SourceStore
}

// NewAmbientalHandler creates an AmbientalHandler backed by the given store.
func NewAmbientalHandler(store SourceStore) *AmbientalHandler {
	return &AmbientalHandler{store: store}
}

// GetDesmatamento handles GET /v1/ambiental/desmatamento.
// Returns the latest INPE DETER deforestation alerts.
func (h *AmbientalHandler) GetDesmatamento(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "inpe_deter")
	if err != nil {
		gatewayError(w, "ambiental", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "DETER deforestation data not yet available")
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.002",
		Data:      rec.Data,
	})
}

// GetProdes handles GET /v1/ambiental/prodes.
// Returns the latest INPE PRODES annual deforestation data.
func (h *AmbientalHandler) GetProdes(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "inpe_prodes")
	if err != nil {
		gatewayError(w, "ambiental", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "PRODES deforestation data not yet available")
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.002",
		Data:      rec.Data,
	})
}
