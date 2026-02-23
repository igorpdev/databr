// Package anp implements collectors for Brazilian fuel price data from ANP
// (Agencia Nacional do Petroleo, Gas Natural e Biocombustiveis).
package anp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/databr/api/internal/domain"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	defaultXLSXURL = "https://www.gov.br/anp/pt-br/assuntos/precos-e-defesa-da-concorrencia/precos/precos-revenda-e-de-distribuicao-combustiveis/shlp/mensal/mensal-brasil-desde-jan2013.xlsx"
)

// knownColumns maps normalized header names to standardized field names.
var knownColumns = map[string]string{
	"regiao":                "regiao",
	"estado":                "estado",
	"produto":               "produto",
	"data inicial":          "data_inicial",
	"data final":            "data_final",
	"preco medio revenda":   "preco_medio_revenda",
	"desvio padrao revenda": "desvio_padrao_revenda",
	"preco minimo revenda":  "preco_minimo_revenda",
	"preco maximo revenda":  "preco_maximo_revenda",
	"margem media revenda":  "margem_media_revenda",
	"coef de variacao revenda": "coef_variacao_revenda",
	"preco medio distribuicao":   "preco_medio_distribuicao",
	"desvio padrao distribuicao": "desvio_padrao_distribuicao",
	"preco minimo distribuicao":  "preco_minimo_distribuicao",
	"preco maximo distribuicao":  "preco_maximo_distribuicao",
	"coef de variacao distribuicao": "coef_variacao_distribuicao",
	"numero de postos pesquisados":  "numero_postos_pesquisados",
}

// CombustiveisCollector fetches the ANP monthly fuel price XLSX and produces
// one SourceRecord per product/date combination.
type CombustiveisCollector struct {
	xlsxURL    string
	httpClient *http.Client
}

// NewCombustiveisCollector creates a new CombustiveisCollector.
// Pass an empty xlsxURL to use the default ANP URL.
func NewCombustiveisCollector(xlsxURL string) *CombustiveisCollector {
	if xlsxURL == "" {
		xlsxURL = defaultXLSXURL
	}
	return &CombustiveisCollector{
		xlsxURL: xlsxURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *CombustiveisCollector) Source() string   { return "anp_combustiveis" }
func (c *CombustiveisCollector) Schedule() string { return "@weekly" }

// Collect downloads the ANP XLSX, parses fuel price rows, and returns one
// SourceRecord per product+date combination. ANP blocks HEAD requests (403),
// so only GET is used.
func (c *CombustiveisCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.xlsxURL, nil)
	if err != nil {
		return nil, fmt.Errorf("anp_combustiveis: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anp_combustiveis: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anp_combustiveis: upstream returned %d", resp.StatusCode)
	}

	// ANP may return HTML instead of XLSX — guard against this.
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !isXLSXContentType(ct) {
		return nil, fmt.Errorf("anp_combustiveis: unexpected Content-Type %q (expected XLSX)", ct)
	}

	// Limit response body to 100 MB to prevent OOM from unexpectedly large files.
	const maxXLSXSize = 100 * 1024 * 1024
	f, err := excelize.OpenReader(io.LimitReader(resp.Body, maxXLSXSize))
	if err != nil {
		return nil, fmt.Errorf("anp_combustiveis: open xlsx: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("anp_combustiveis: no sheets found in XLSX")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("anp_combustiveis: read rows: %w", err)
	}

	if len(rows) < 2 {
		// Only header or empty — no data rows.
		return nil, nil
	}

	// Build column index from header row.
	colMap := buildColumnMap(rows[0])
	if len(colMap) == 0 {
		return nil, fmt.Errorf("anp_combustiveis: could not identify any known columns in header")
	}

	now := time.Now().UTC()
	var records []domain.SourceRecord

	for _, row := range rows[1:] {
		data := extractRowData(row, colMap)
		if len(data) == 0 {
			continue
		}

		produto, _ := data["produto"].(string)
		dataInicial, _ := data["data_inicial"].(string)
		if produto == "" || dataInicial == "" {
			continue
		}

		recordKey := normalizeKey(produto) + "_" + normalizeDate(dataInicial)

		records = append(records, domain.SourceRecord{
			Source:    "anp_combustiveis",
			RecordKey: recordKey,
			Data:      data,
			FetchedAt: now,
		})
	}

	return records, nil
}

// buildColumnMap reads the header row and returns a map from column index to
// the standardized field name.
func buildColumnMap(header []string) map[int]string {
	colMap := make(map[int]string)
	for i, h := range header {
		normalized := normalizeHeader(h)
		if fieldName, ok := knownColumns[normalized]; ok {
			colMap[i] = fieldName
		}
	}
	return colMap
}

// extractRowData reads a single data row using the column map and returns a
// map of standardized field name to cell value.
func extractRowData(row []string, colMap map[int]string) map[string]any {
	data := make(map[string]any)
	for idx, fieldName := range colMap {
		if idx < len(row) {
			val := strings.TrimSpace(row[idx])
			if val != "" {
				data[fieldName] = val
			}
		}
	}
	return data
}

// normalizeHeader strips accents, lowercases, and collapses whitespace so that
// column names like "PRECO MEDIO REVENDA" or "Preco Medio Revenda" all match.
func normalizeHeader(s string) string {
	s = stripAccents(s)
	s = strings.ToLower(s)
	// Collapse multiple spaces into one.
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// normalizeKey produces a URL-safe record key fragment from a product name:
// strips accents, lowercases, replaces spaces/special chars with underscores.
func normalizeKey(s string) string {
	s = stripAccents(s)
	s = strings.ToLower(s)
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore && b.Len() > 0 {
				b.WriteRune('_')
				prevUnderscore = true
			}
		}
	}
	result := b.String()
	return strings.TrimRight(result, "_")
}

// normalizeDate tries to extract a YYYY-MM date string from various formats
// such as "2026-01-01", "01/01/2026", etc. Falls back to the raw value.
func normalizeDate(s string) string {
	s = strings.TrimSpace(s)

	// Try ISO: "2026-01-01" or "2026-01"
	if len(s) >= 7 && s[4] == '-' {
		return s[:7] // "2026-01"
	}

	// Try BR: "01/01/2026"
	if len(s) >= 10 && s[2] == '/' && s[5] == '/' {
		return s[6:10] + "-" + s[3:5] // "2026-01"
	}

	return s
}

// stripAccents removes Unicode combining diacritical marks from s.
func stripAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return result
}

// isXLSXContentType checks whether the Content-Type indicates an XLSX file.
func isXLSXContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "spreadsheet") ||
		strings.Contains(ct, "xlsx") ||
		strings.Contains(ct, "octet-stream") ||
		strings.Contains(ct, "application/zip")
}
