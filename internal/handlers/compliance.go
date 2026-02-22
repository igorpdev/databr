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
