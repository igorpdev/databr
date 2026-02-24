package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

const dataSUSBaseURL = "https://apidadosabertos.saude.gov.br"

// DATASUSHandler handles requests for /v1/saude/datasus/*.
// On-demand proxy to the DATASUS open data API (no DB required).
type DATASUSHandler struct {
	baseURL string
	client  *http.Client
}

// NewDATASUSHandler creates a handler that proxies to the DATASUS open data API.
func NewDATASUSHandler() *DATASUSHandler {
	return &DATASUSHandler{
		baseURL: dataSUSBaseURL,
		client:  newHTTPClient(30 * time.Second),
	}
}

// GetEstabelecimento handles GET /v1/saude/estabelecimentos/{cnes}.
// Returns a single health establishment by its CNES code.
func (h *DATASUSHandler) GetEstabelecimento(w http.ResponseWriter, r *http.Request) {
	cnes := chi.URLParam(r, "cnes")
	if cnes == "" {
		jsonError(w, http.StatusBadRequest, "cnes is required")
		return
	}

	url := fmt.Sprintf("%s/cnes/estabelecimentos/%s", h.baseURL, cnes)

	var result map[string]any
	if _, err := fetchJSON(r.Context(), h.client, url, nil, &result); err != nil {
		gatewayError(w, "datasus_estabelecimentos", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datasus_cnes",
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      result,
	})
}

// GetEstabelecimentos handles GET /v1/saude/estabelecimentos?municipio=IBGE&uf=XX&limit=N.
// Searches health establishments by municipality code and/or UF.
func (h *DATASUSHandler) GetEstabelecimentos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	municipio := q.Get("municipio")
	uf := q.Get("uf")

	if municipio == "" && uf == "" {
		jsonError(w, http.StatusBadRequest, "municipio (IBGE code) or uf (state) is required")
		return
	}

	limit, _ := parsePagination(r)

	// Build upstream query
	upstream := fmt.Sprintf("%s/cnes/estabelecimentos?limit=%d", h.baseURL, limit)
	if municipio != "" {
		upstream += "&codigo_municipio=" + municipio
	}
	if uf != "" {
		upstream += "&codigo_uf=" + uf
	}

	var result struct {
		Estabelecimentos []map[string]any `json:"estabelecimentos"`
	}
	if _, err := fetchJSON(r.Context(), h.client, upstream, nil, &result); err != nil {
		gatewayError(w, "datasus_estabelecimentos", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datasus_cnes",
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"estabelecimentos": result.Estabelecimentos,
			"total":            len(result.Estabelecimentos),
		},
	})
}
