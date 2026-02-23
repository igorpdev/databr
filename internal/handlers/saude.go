package handlers

import (
	"net/http"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// SaudeHandler handles requests for /v1/saude/*.
type SaudeHandler struct {
	store SourceStore
}

// NewSaudeHandler creates a SaudeHandler backed by the given SourceStore.
func NewSaudeHandler(store SourceStore) *SaudeHandler {
	return &SaudeHandler{store: store}
}

// GetMedicamento handles GET /v1/saude/medicamentos/{registro}.
// {registro} is the NUMERO_REGISTRO_PRODUTO from ANVISA open data.
func (h *SaudeHandler) GetMedicamento(w http.ResponseWriter, r *http.Request) {
	registro := chi.URLParam(r, "registro")

	rec, err := h.store.FindOne(r.Context(), "anvisa_medicamentos", registro)
	if err != nil {
		gatewayError(w, "saude", err)
		return
	}
	if rec == nil {
		jsonError(w, http.StatusNotFound, "medicamento não encontrado: "+registro)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}
