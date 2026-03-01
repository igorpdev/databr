package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

const bndesCKANBase = "https://dadosabertos.bndes.gov.br"
const bndesPkgID = "operacoes-de-credito-direto-e-indireto"

// BNDESHandler handles on-demand BNDES open data requests.
type BNDESHandler struct {
	httpClient *http.Client
	baseURL    string
}

// NewBNDESHandler creates a BNDESHandler using the BNDES CKAN open data API.
func NewBNDESHandler() *BNDESHandler {
	return &BNDESHandler{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    bndesCKANBase,
	}
}

// NewBNDESHandlerWithBaseURL creates a BNDESHandler with a custom base URL (for testing).
func NewBNDESHandlerWithBaseURL(baseURL string) *BNDESHandler {
	return &BNDESHandler{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    baseURL,
	}
}

// GetOperacoes handles GET /v1/bndes/operacoes/{cnpj}?n=20
// Returns BNDES financing operations for a given CNPJ.
// Uses the BNDES CKAN open data portal (no API key required).
func (h *BNDESHandler) GetOperacoes(w http.ResponseWriter, r *http.Request) {
	cnpj := reDigits.ReplaceAllString(chi.URLParam(r, "cnpj"), "")
	if len(cnpj) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos")
		return
	}
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		var v int
		if _, err := fmt.Sscanf(raw, "%d", &v); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	// Step 1: get current resource_id from CKAN package
	pkgURL := fmt.Sprintf("%s/api/3/action/package_show?id=%s", h.baseURL, bndesPkgID)
	pkgReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, pkgURL, nil)
	if err != nil {
		internalError(w, "bndes", err)
		return
	}
	pkgResp, err := h.httpClient.Do(pkgReq)
	if err != nil {
		gatewayError(w, "bndes", err)
		return
	}
	defer pkgResp.Body.Close()

	var pkgResult struct {
		Success bool `json:"success"`
		Result  struct {
			Resources []struct {
				ID string `json:"id"`
			} `json:"resources"`
		} `json:"result"`
	}
	if err := json.NewDecoder(pkgResp.Body).Decode(&pkgResult); err != nil || !pkgResult.Success || len(pkgResult.Result.Resources) == 0 {
		jsonError(w, http.StatusBadGateway, "BNDES: não foi possível obter resource_id do dataset")
		return
	}
	resourceID := pkgResult.Result.Resources[0].ID

	// Step 2: search datastore by CNPJ
	searchURL := fmt.Sprintf(
		"%s/api/3/action/datastore_search?resource_id=%s&q=%s&limit=%d",
		h.baseURL, resourceID, cnpj, n,
	)
	searchReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, searchURL, nil)
	if err != nil {
		internalError(w, "bndes", err)
		return
	}
	searchResp, err := h.httpClient.Do(searchReq)
	if err != nil {
		gatewayError(w, "bndes", err)
		return
	}
	defer searchResp.Body.Close()

	var searchResult struct {
		Success bool `json:"success"`
		Result  struct {
			Records []any `json:"records"`
			Total   int   `json:"total"`
		} `json:"result"`
	}
	if err := json.NewDecoder(searchResp.Body).Decode(&searchResult); err != nil || !searchResult.Success {
		jsonError(w, http.StatusBadGateway, "BNDES: erro ao buscar operações")
		return
	}

	if len(searchResult.Result.Records) == 0 {
		jsonError(w, http.StatusNotFound, "Nenhuma operação BNDES encontrada para CNPJ "+cnpj)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "bndes_operacoes",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"cnpj":      cnpj,
			"operacoes": searchResult.Result.Records,
			"total":     searchResult.Result.Total,
		},
	})
}
