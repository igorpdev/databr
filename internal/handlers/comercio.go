package handlers

import (
	"net/http"
)

// ComercioHandler handles requests for /v1/comercio/*.
type ComercioHandler struct {
	store SourceStore
}

// NewComercioHandler creates a ComercioHandler backed by the given SourceStore.
func NewComercioHandler(store SourceStore) *ComercioHandler {
	return &ComercioHandler{store: store}
}

// GetExportacoes handles GET /v1/comercio/exportacoes.
func (h *ComercioHandler) GetExportacoes(w http.ResponseWriter, r *http.Request) {
	serveLatestAll(w, r, h.store, "comex_exportacoes", "exportacoes")
}

// GetImportacoes handles GET /v1/comercio/importacoes.
func (h *ComercioHandler) GetImportacoes(w http.ResponseWriter, r *http.Request) {
	serveLatestAll(w, r, h.store, "comex_importacoes", "importacoes")
}
