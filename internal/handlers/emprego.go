package handlers

import (
	"net/http"
)

// EmpregoHandler handles requests for /v1/emprego/*.
type EmpregoHandler struct {
	store SourceStore
}

// NewEmpregoHandler creates an EmpregoHandler backed by the given SourceStore.
func NewEmpregoHandler(store SourceStore) *EmpregoHandler {
	return &EmpregoHandler{store: store}
}

// GetRAIS handles GET /v1/emprego/rais.
func (h *EmpregoHandler) GetRAIS(w http.ResponseWriter, r *http.Request) {
	serveLatestAll(w, r, h.store, "rais_emprego", "registros")
}

// GetCAGED handles GET /v1/emprego/caged.
func (h *EmpregoHandler) GetCAGED(w http.ResponseWriter, r *http.Request) {
	serveLatestAll(w, r, h.store, "caged_emprego", "registros")
}
