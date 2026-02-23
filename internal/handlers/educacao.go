package handlers

import (
	"net/http"
)

// EducacaoHandler handles requests for /v1/educacao/*.
type EducacaoHandler struct {
	store SourceStore
}

// NewEducacaoHandler creates an EducacaoHandler backed by the given SourceStore.
func NewEducacaoHandler(store SourceStore) *EducacaoHandler {
	return &EducacaoHandler{store: store}
}

// GetCensoEscolar handles GET /v1/educacao/censo-escolar.
func (h *EducacaoHandler) GetCensoEscolar(w http.ResponseWriter, r *http.Request) {
	serveLatestAll(w, r, h.store, "inep_censo_escolar", "registros")
}
