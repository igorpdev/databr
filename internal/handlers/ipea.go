package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// IPEAHandler handles requests for /v1/ipea/*.
type IPEAHandler struct {
	httpClient *http.Client
}

// NewIPEAHandler creates a new IPEAHandler with a default HTTP client (15s timeout).
func NewIPEAHandler() *IPEAHandler {
	return &IPEAHandler{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// NewIPEAHandlerWithClient creates a new IPEAHandler using the provided HTTP client.
// Useful for testing with a custom transport that redirects to a mock server.
func NewIPEAHandlerWithClient(client *http.Client) *IPEAHandler {
	return &IPEAHandler{httpClient: client}
}

// GetSerie handles GET /v1/ipea/serie/{codigo}.
// Returns the most recent N values of the given IPEAData series.
//
// Path param: codigo — IPEA series code (e.g., "BM12_TJOVER12", "PRECOS12_IPCA12", "SCN10_TRIBFBCF10")
// Optional query params:
//   - n: number of values to return (default 24, max 120)
//   - desde: ISO date to filter from (e.g. "2024-01-01"); defaults to 2 years ago
func (h *IPEAHandler) GetSerie(w http.ResponseWriter, r *http.Request) {
	codigo := chi.URLParam(r, "codigo")

	// Parse n (default 24, clamp 1..120).
	n := 24
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			n = v
		}
	}
	if n < 1 {
		n = 1
	}
	if n > 120 {
		n = 120
	}

	// Compute desde: use query param if provided, otherwise 2 years ago.
	desde := r.URL.Query().Get("desde")
	if desde == "" {
		t := time.Now().AddDate(-2, 0, 0)
		// OData $filter date literal — use timezone offset Brazil -03:00.
		desde = t.Format("2006-01-02") + "T00:00:00-03:00"
	} else {
		// If caller provided a plain date (YYYY-MM-DD), append the time portion.
		if len(desde) == 10 {
			desde = desde + "T00:00:00-03:00"
		}
	}

	// Build the OData $filter URL.
	// The filter expression must be percent-encoded so that the space between
	// the operator and the date literal is transmitted as %20 — Go's net/http
	// server (and some strict clients) reject URLs with raw spaces in the query.
	filterExpr := fmt.Sprintf("VALDATA gt %s", desde)
	q := url.Values{}
	q.Set("$filter", filterExpr)
	upstreamURL := fmt.Sprintf(
		"http://ipeadata.gov.br/api/odata4/ValoresSerie(SERCODIGO='%s')?%s",
		codigo, q.Encode(),
	)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Erro ao construir requisição IPEAData: "+err.Error())
		return
	}
	// IPEAData OData v4: must use Accept header — $format=json returns 400.
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar IPEAData: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("IPEAData retornou status %d", resp.StatusCode))
		return
	}

	var envelope struct {
		Value []map[string]any `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta IPEAData: "+err.Error())
		return
	}

	if len(envelope.Value) == 0 {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("Série IPEAData não encontrada ou sem dados: %s", codigo))
		return
	}

	// Take the last min(n, len) items — most recent values are at the end.
	values := envelope.Value
	start := len(values) - n
	if start < 0 {
		start = 0
	}
	lastN := values[start:]

	respond(w, r, domain.APIResponse{
		Source:   "ipea_" + codigo,
		CostUSDC: "0.001",
		Data: map[string]any{
			"serie":   codigo,
			"valores": lastN,
			"total":   len(lastN),
		},
	})
}
