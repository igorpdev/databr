package handlers

import (
	"context"
	"net/http"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// ComplianceFetcher retrieves compliance (CEIS/CNEP) data for a given CNPJ.
type ComplianceFetcher interface {
	FetchByCNPJ(ctx context.Context, cnpjNum string) ([]domain.SourceRecord, error)
	// FetchGranularByCNPJ fetches a specific compliance list ("ceis", "cnep", or "cepim").
	FetchGranularByCNPJ(ctx context.Context, cnpjNum, list string) ([]domain.SourceRecord, error)
}

// ComplianceHandler handles /v1/compliance/* and /v1/empresas/{cnpj}/compliance.
type ComplianceHandler struct {
	fetcher ComplianceFetcher
}

// NewComplianceHandler creates a compliance handler.
func NewComplianceHandler(fetcher ComplianceFetcher) *ComplianceHandler {
	return &ComplianceHandler{fetcher: fetcher}
}

// GetCompliance handles GET /v1/compliance/{cnpj} and GET /v1/empresas/{cnpj}/compliance.
func (h *ComplianceHandler) GetCompliance(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, http.StatusNotFound, "Compliance data not found for CNPJ "+normalized)
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.005",
		Data:      rec.Data,
	})
}

// fetchGranular is a shared helper for the three granular compliance handlers.
func (h *ComplianceHandler) fetchGranular(w http.ResponseWriter, r *http.Request, list string) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if len(normalized) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ must have 14 digits")
		return
	}

	records, err := h.fetcher.FetchGranularByCNPJ(r.Context(), normalized, list)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No "+list+" data found for CNPJ "+normalized)
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetCEIS handles GET /v1/compliance/ceis/{cnpj}.
func (h *ComplianceHandler) GetCEIS(w http.ResponseWriter, r *http.Request) {
	h.fetchGranular(w, r, "ceis")
}

// GetCNEP handles GET /v1/compliance/cnep/{cnpj}.
func (h *ComplianceHandler) GetCNEP(w http.ResponseWriter, r *http.Request) {
	h.fetchGranular(w, r, "cnep")
}

// GetCEPIM handles GET /v1/compliance/cepim/{cnpj}.
func (h *ComplianceHandler) GetCEPIM(w http.ResponseWriter, r *http.Request) {
	h.fetchGranular(w, r, "cepim")
}
