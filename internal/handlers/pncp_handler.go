package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/databr/api/internal/domain"
)

// PNCPHandler handles requests for /v1/pncp/*.
type PNCPHandler struct {
	httpClient *http.Client
}

// NewPNCPHandler creates a new PNCPHandler with a default HTTP client.
func NewPNCPHandler() *PNCPHandler {
	return &PNCPHandler{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// NewPNCPHandlerWithClient creates a new PNCPHandler using the provided HTTP client.
func NewPNCPHandlerWithClient(client *http.Client) *PNCPHandler {
	return &PNCPHandler{httpClient: client}
}

// GetOrgaos handles GET /v1/pncp/orgaos.
// Returns the list of public procurement organs registered in PNCP.
// Optional query params: pagina (default 1), n (per page, default 20, max 100).
func (h *PNCPHandler) GetOrgaos(w http.ResponseWriter, r *http.Request) {
	pagina := 1
	if raw := r.URL.Query().Get("pagina"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			pagina = v
		}
	}
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	upURL := fmt.Sprintf(
		"https://pncp.gov.br/api/pncp/v1/orgaos?pagina=%d&tamanhoPagina=%d",
		pagina, n,
	)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "pncp", err)
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "pncp", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("pncp", resp.StatusCode, body))
		return
	}

	var orgaos []any
	if err := json.NewDecoder(resp.Body).Decode(&orgaos); err != nil {
		gatewayError(w, "pncp", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "pncp_orgaos",
		CostUSDC: "0.001",
		Data:     map[string]any{"orgaos": orgaos, "total": len(orgaos), "pagina": pagina},
	})
}
