package handlers

import (
	"bufio"
	"encoding/csv"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
)

const ansOperadorasURL = "https://dadosabertos.ans.gov.br/FTP/PDA/operadoras_de_plano_de_saude_ativas/Relatorio_cadop.csv"

// ANSHandler handles requests for /v1/saude/planos.
// Data source: ANS Dados Abertos — Operadoras de Plano de Saúde Ativas
// CSV updated daily at dadosabertos.ans.gov.br (UTF-8, semicolon-separated, ~339 KB).
type ANSHandler struct {
	httpClient *http.Client
}

// NewANSHandler creates an ANSHandler with a default HTTP client.
func NewANSHandler() *ANSHandler {
	return &ANSHandler{httpClient: &http.Client{Timeout: 30 * time.Second}}
}

// NewANSHandlerWithClient creates an ANSHandler using the provided HTTP client.
func NewANSHandlerWithClient(client *http.Client) *ANSHandler {
	return &ANSHandler{httpClient: client}
}

// GetPlanos handles GET /v1/saude/planos.
//
// Returns the list of health plan operators active in ANS.
// Optional query params:
//   - uf=SP      — filter by state (two-letter abbreviation, case-insensitive)
//   - modalidade — filter by modality substring (e.g. "Medicina de Grupo", "Cooperativa")
//   - n          — max number of results to return (default 20, max 200)
func (h *ANSHandler) GetPlanos(w http.ResponseWriter, r *http.Request) {
	uf := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("uf")))
	modalidade := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("modalidade")))
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 200 {
			n = v
		}
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, ansOperadorasURL, nil)
	if err != nil {
		internalError(w, "ans", err)
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "ans", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("ANS", resp.StatusCode, body))
		return
	}

	// Parse CSV — the file uses semicolon as separator and UTF-8 encoding.
	csvReader := csv.NewReader(bufio.NewReader(resp.Body))
	csvReader.Comma = ';'
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true

	// Read header row.
	headers, err := csvReader.Read()
	if err != nil {
		gatewayError(w, "ans", err)
		return
	}
	// Normalize headers to lowercase.
	for i, h := range headers {
		headers[i] = strings.ToLower(strings.TrimSpace(h))
	}

	// Build column index map.
	colIdx := make(map[string]int, len(headers))
	for i, h := range headers {
		colIdx[h] = i
	}

	var operadoras []map[string]any
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Warn("ANS CSV malformed row", "error", err)
			continue
		}

		// Apply UF filter.
		if uf != "" {
			colUF, ok := colIdx["uf"]
			if !ok || colUF >= len(row) {
				continue
			}
			if strings.ToUpper(strings.TrimSpace(row[colUF])) != uf {
				continue
			}
		}

		// Apply modalidade filter.
		if modalidade != "" {
			colMod, ok := colIdx["modalidade"]
			if !ok || colMod >= len(row) {
				continue
			}
			if !strings.Contains(strings.ToLower(row[colMod]), modalidade) {
				continue
			}
		}

		// Build record map.
		rec := make(map[string]any, len(headers))
		for i, h := range headers {
			if i < len(row) {
				rec[h] = strings.TrimSpace(row[i])
			}
		}
		operadoras = append(operadoras, rec)

		if len(operadoras) >= n {
			break
		}
	}

	if operadoras == nil {
		operadoras = []map[string]any{}
	}

	respond(w, r, domain.APIResponse{
		Source:   "ans_operadoras",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"operadoras": operadoras,
			"total":      len(operadoras),
			"filtros": map[string]any{
				"uf":         uf,
				"modalidade": modalidade,
				"n":          n,
			},
		},
	})
}
