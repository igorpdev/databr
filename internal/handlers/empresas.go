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

// GetSimples handles GET /v1/empresas/{cnpj}/simples.
// Extracts the Simples Nacional and MEI status from the CNPJ record returned
// by minhareceita.org (keys "simples" and "mei" inside Data).
func (h *EmpresasHandler) GetSimples(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if len(normalized) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ must have 14 digits")
		return
	}

	records, err := h.fetcher.FetchByCNPJ(r.Context(), normalized)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}

	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "CNPJ not found")
		return
	}

	rec := records[0]
	simples := rec.Data["simples"]
	mei := rec.Data["mei"]

	if simples == nil && mei == nil {
		jsonError(w, http.StatusNotFound, "Dados do Simples Nacional não disponíveis para este CNPJ")
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "cnpj_simples",
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      map[string]any{"cnpj": normalized, "simples": simples, "mei": mei},
	})
}

// GetSocios handles GET /v1/empresas/{cnpj}/socios.
// Calls FetchByCNPJ and extracts the "qsa" field (quadro societário) from the returned data.
func (h *EmpresasHandler) GetSocios(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if len(normalized) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ must have 14 digits")
		return
	}

	records, err := h.fetcher.FetchByCNPJ(r.Context(), normalized)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}

	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "CNPJ not found")
		return
	}

	rec := records[0]
	qsa, ok := rec.Data["qsa"]
	if !ok || qsa == nil {
		jsonError(w, http.StatusNotFound, "Nenhum sócio encontrado")
		return
	}

	// qsa might be an empty slice — treat that as not found too.
	if qsaSlice, ok := qsa.([]any); ok && len(qsaSlice) == 0 {
		jsonError(w, http.StatusNotFound, "Nenhum sócio encontrado")
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      map[string]any{"cnpj": normalized, "qsa": qsa},
	})
}
