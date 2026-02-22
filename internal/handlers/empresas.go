// Package handlers contains HTTP handlers for the DataBR REST API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// CNPJFetcher is the interface required by EmpresasHandler.
// Allows injection of real collector or test stub.
type CNPJFetcher interface {
	FetchByCNPJ(ctx context.Context, cnpjNum string) ([]domain.SourceRecord, error)
}

// EmpresasHandler handles requests for /v1/empresas/*.
type EmpresasHandler struct {
	fetcher CNPJFetcher
}

// NewEmpresasHandler creates a new handler with the given CNPJ fetcher.
func NewEmpresasHandler(fetcher CNPJFetcher) *EmpresasHandler {
	return &EmpresasHandler{fetcher: fetcher}
}

// GetEmpresa handles GET /v1/empresas/{cnpj}.
func (h *EmpresasHandler) GetEmpresa(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if len(normalized) != 14 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "CNPJ must have 14 digits",
		})
		return
	}

	records, err := h.fetcher.FetchByCNPJ(r.Context(), normalized)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	if len(records) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "CNPJ not found",
		})
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		Cached:    false,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}
