package handlers

import (
	"net/http"

	"github.com/databr/api/internal/domain"
)

// TitulosHandler handles GET /v1/tesouro/titulos using a store-backed approach.
// It is separate from TesouroHandler so main.go can wire it independently
// without modifying the existing NewTesouroHandler signature.
type TitulosHandler struct {
	store SourceStore
}

// NewTitulosHandler creates a Titulos handler backed by the given store.
func NewTitulosHandler(store SourceStore) *TitulosHandler {
	return &TitulosHandler{store: store}
}

// GetTitulos handles GET /v1/tesouro/titulos.
// Returns all available Tesouro Direto bonds with current prices and rates.
func (h *TitulosHandler) GetTitulos(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "tesouro_titulos")
	if err != nil {
		gatewayError(w, "titulos", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "Títulos do Tesouro Direto não disponíveis ainda")
		return
	}

	titulos := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		titulos = append(titulos, rec.Data)
	}

	respond(w, r, domain.APIResponse{
		Source:    "tesouro_titulos",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  "0.001",
		Data:      map[string]any{"titulos": titulos},
	})
}
