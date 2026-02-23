package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
	"golang.org/x/text/encoding/charmap"
)

// TSEExtrasHandler handles on-demand requests for TSE election data
// downloaded directly from the TSE CDN ZIP archives.
type TSEExtrasHandler struct {
	httpClient *http.Client
	baseURL    string
}

// NewTSEExtrasHandler creates a TSEExtrasHandler with default HTTP client and TSE CDN base URL.
func NewTSEExtrasHandler() *TSEExtrasHandler {
	return &TSEExtrasHandler{
		httpClient: &http.Client{Timeout: 120 * time.Second}, // large ZIP files
		baseURL:    "https://cdn.tse.jus.br/estatistica/sead/odsele",
	}
}

// NewTSEExtrasHandlerWithClient creates a TSEExtrasHandler using the provided HTTP client and base URL.
// Useful for testing.
func NewTSEExtrasHandlerWithClient(client *http.Client, baseURL string) *TSEExtrasHandler {
	return &TSEExtrasHandler{
		httpClient: client,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// downloadZip fetches a ZIP archive from the given URL and returns its raw bytes.
func (h *TSEExtrasHandler) downloadZip(r *http.Request, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	body, err := limitedReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

// parseZipCSV opens a ZIP archive and parses all CSV files inside it using a Latin-1 decoder.
// Returns up to maxRecords rows as []map[string]any (headers lowercased).
func parseZipCSV(zipData []byte, maxRecords int) ([]map[string]any, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	var allRows []map[string]any
	for _, f := range r.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}

		// TSE CSVs use Latin-1 encoding — decode to UTF-8 before CSV parsing.
		decoder := charmap.ISO8859_1.NewDecoder()
		utf8Reader := decoder.Reader(rc)
		csvReader := csv.NewReader(utf8Reader)
		csvReader.Comma = ';'
		csvReader.LazyQuotes = true

		headers, err := csvReader.Read()
		if err != nil {
			rc.Close()
			continue
		}
		for i, h := range headers {
			headers[i] = strings.ToLower(strings.TrimSpace(h))
		}

		for {
			if maxRecords > 0 && len(allRows) >= maxRecords {
				break
			}
			row, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("WARN: TSE CSV malformed row in %s: %v", f.Name, err)
				continue
			}
			m := make(map[string]any, len(headers))
			for i, h := range headers {
				if i < len(row) {
					m[h] = strings.TrimSpace(row[i])
				}
			}
			allRows = append(allRows, m)
		}
		rc.Close()

		if maxRecords > 0 && len(allRows) >= maxRecords {
			break
		}
	}
	return allRows, nil
}

// parseLimitN parses the ?n= query param, applying defaultN and maxN bounds.
func parseLimitN(r *http.Request, defaultN, maxN int) int {
	n := defaultN
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			if v > maxN {
				v = maxN
			}
			n = v
		}
	}
	return n
}

// latestElectionYear returns the most recent election year (even year <= current year).
// Brazilian elections are held every 2 years on even years.
func latestElectionYear() int {
	y := time.Now().Year()
	if y%2 != 0 {
		y--
	}
	return y
}

// parseAno parses the ?ano= query param, defaulting to the latest election year.
func parseAno(r *http.Request) int {
	ano := latestElectionYear()
	if raw := r.URL.Query().Get("ano"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 2000 && v <= 2100 {
			ano = v
		}
	}
	return ano
}

// GetBens handles GET /v1/eleicoes/bens?ano=2024&n=30
// Downloads TSE consulta_bem_candidato ZIP and returns the first n records.
func (h *TSEExtrasHandler) GetBens(w http.ResponseWriter, r *http.Request) {
	ano := parseAno(r)
	n := parseLimitN(r, 30, 200)

	zipURL := fmt.Sprintf("%s/consulta_bem_candidato/consulta_bem_candidato_%d.zip", h.baseURL, ano)
	zipData, err := h.downloadZip(r, zipURL)
	if err != nil {
		gatewayError(w, "tse_bens", err)
		return
	}

	rows, err := parseZipCSV(zipData, n)
	if err != nil {
		gatewayError(w, "tse_bens", err)
		return
	}

	if len(rows) == 0 {
		jsonError(w, http.StatusNotFound, "tse_bens: no records found for ano "+strconv.Itoa(ano))
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "tse_bens",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.001",
		Data:      map[string]any{"bens": rows, "total": len(rows), "ano": ano},
	})
}

// GetDoacoes handles GET /v1/eleicoes/doacoes?ano=2024&n=30
// Downloads TSE receitas_candidatos ZIP and returns the first n donation records.
func (h *TSEExtrasHandler) GetDoacoes(w http.ResponseWriter, r *http.Request) {
	ano := parseAno(r)
	n := parseLimitN(r, 30, 200)

	zipURL := fmt.Sprintf("%s/receitas_candidatos/receitas_candidatos_%d.zip", h.baseURL, ano)
	zipData, err := h.downloadZip(r, zipURL)
	if err != nil {
		gatewayError(w, "tse_doacoes", err)
		return
	}

	rows, err := parseZipCSV(zipData, n)
	if err != nil {
		gatewayError(w, "tse_doacoes", err)
		return
	}

	if len(rows) == 0 {
		jsonError(w, http.StatusNotFound, "tse_doacoes: no records found for ano "+strconv.Itoa(ano))
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "tse_doacoes",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.001",
		Data:      map[string]any{"doacoes": rows, "total": len(rows), "ano": ano},
	})
}

// GetResultados handles GET /v1/eleicoes/resultados?ano=2024&n=30
// Downloads TSE votacao_candidato_munzona ZIP and returns the first n result records.
func (h *TSEExtrasHandler) GetResultados(w http.ResponseWriter, r *http.Request) {
	ano := parseAno(r)
	n := parseLimitN(r, 30, 200)

	zipURL := fmt.Sprintf("%s/votacao_candidato_munzona/votacao_candidato_munzona_%d.zip", h.baseURL, ano)
	zipData, err := h.downloadZip(r, zipURL)
	if err != nil {
		gatewayError(w, "tse_resultados", err)
		return
	}

	rows, err := parseZipCSV(zipData, n)
	if err != nil {
		gatewayError(w, "tse_resultados", err)
		return
	}

	if len(rows) == 0 {
		jsonError(w, http.StatusNotFound, "tse_resultados: no records found for ano "+strconv.Itoa(ano))
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "tse_resultados",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.001",
		Data:      map[string]any{"resultados": rows, "total": len(rows), "ano": ano},
	})
}

// ipeaSeriesInfo holds metadata for a single IPEA data series used in combustíveis.
type ipeaSeriesInfo struct {
	Codigo    string `json:"codigo"`
	Nome      string `json:"nome"`
	Unidade   string `json:"unidade"`
	Descricao string `json:"descricao"`
}

// combustiveisSeries lists the ANP fuel price series available in IPEADATA.
var combustiveisSeries = []ipeaSeriesInfo{
	{
		Codigo:    "ANP_PRALCO",
		Nome:      "alcool_hidratado",
		Unidade:   "R$/m³",
		Descricao: "Preço médio - álcool hidratado - metro cúbico",
	},
	{
		Codigo:    "ANP_PRGASOL",
		Nome:      "gasolina",
		Unidade:   "R$/m³",
		Descricao: "Preço médio - gasolina - metro cúbico",
	},
	{
		Codigo:    "ANP_PRGLP",
		Nome:      "glp_gas",
		Unidade:   "R$/t",
		Descricao: "Preço médio - gás GLP - tonelada",
	},
	{
		Codigo:    "ANP_PROLDIE",
		Nome:      "oleo_diesel",
		Unidade:   "R$/m³",
		Descricao: "Preço médio - óleo diesel - metro cúbico",
	},
}

// ipeataValor is the struct for a single IPEADATA value.
type ipeataValor struct {
	SerCodigo string  `json:"SERCODIGO"`
	ValData   string  `json:"VALDATA"`
	ValValor  float64 `json:"VALVALOR"`
}

// ipeataResponse is the IPEADATA OData response envelope.
type ipeataResponse struct {
	Value []ipeataValor `json:"value"`
}

// fetchIPEASeries retrieves the last n values for one IPEA series code.
func (h *TSEExtrasHandler) fetchIPEASeries(r *http.Request, serCodigo string, n int) ([]ipeataValor, error) {
	// IMPORTANT: IPEADATA requires Accept: application/json header (not $format=json in URL).
	// Base URL must use HTTP (not HTTPS) — see project CLAUDE.md.
	upURL := fmt.Sprintf(
		"http://www.ipeadata.gov.br/api/odata4/ValoresSerie(SERCODIGO='%s')",
		serCodigo,
	)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		log.Printf("WARN: IPEADATA upstream error (HTTP %d): %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("upstream service temporarily unavailable")
	}

	var result ipeataResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// IPEADATA returns data in chronological order; take the last n values.
	vals := result.Value
	if n > 0 && len(vals) > n {
		vals = vals[len(vals)-n:]
	}
	return vals, nil
}

// GetCombustiveis handles GET /v1/energia/combustiveis?n=12
// Returns ANP fuel price series from IPEADATA (Option C fallback — always works).
// ANP OLINDA is unreachable externally; IPEADATA provides annual ANP price series.
func (h *TSEExtrasHandler) GetCombustiveis(w http.ResponseWriter, r *http.Request) {
	n := parseLimitN(r, 12, 100)

	type combustivelEntry struct {
		Codigo    string       `json:"codigo"`
		Nome      string       `json:"nome"`
		Unidade   string       `json:"unidade"`
		Descricao string       `json:"descricao"`
		Valores   []map[string]any `json:"valores"`
	}

	var combustiveis []combustivelEntry

	for _, series := range combustiveisSeries {
		vals, err := h.fetchIPEASeries(r, series.Codigo, n)
		if err != nil {
			// Non-fatal: skip failing series, log server-side, return generic message.
			log.Printf("ERROR: anp_combustiveis: series %s: %v", series.Codigo, err)
			combustiveis = append(combustiveis, combustivelEntry{
				Codigo:    series.Codigo,
				Nome:      series.Nome,
				Unidade:   series.Unidade,
				Descricao: series.Descricao,
				Valores:   []map[string]any{{"erro": "upstream service temporarily unavailable"}},
			})
			continue
		}

		rows := make([]map[string]any, 0, len(vals))
		for _, v := range vals {
			rows = append(rows, map[string]any{
				"data":  v.ValData,
				"valor": v.ValValor,
			})
		}
		combustiveis = append(combustiveis, combustivelEntry{
			Codigo:    series.Codigo,
			Nome:      series.Nome,
			Unidade:   series.Unidade,
			Descricao: series.Descricao,
			Valores:   rows,
		})
	}

	if len(combustiveis) == 0 {
		jsonError(w, http.StatusBadGateway, "anp_combustiveis: nenhuma série disponível")
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "anp_combustiveis",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.001",
		Data: map[string]any{
			"combustiveis": combustiveis,
			"fonte":        "ipeadata",
			"periodicidade": "anual",
			"nota":         "Séries ANP de preço médio via IPEADATA. Periodicidade anual.",
		},
	})
}
