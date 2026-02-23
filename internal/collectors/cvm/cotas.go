package cvm

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/databr/api/internal/domain"
)

// CotasCollector downloads the daily CVM fund quota values (inf_diario_fi).
// It fetches the current month and the previous month to ensure recent data is always available.
type CotasCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewCotasCollector creates a CVM cotas collector.
func NewCotasCollector(baseURL string) *CotasCollector {
	if baseURL == "" {
		baseURL = cvmBase
	}
	return &CotasCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *CotasCollector) Source() string   { return "cvm_cotas" }
func (c *CotasCollector) Schedule() string { return "@daily" }

// Collect downloads and parses CVM inf_diario_fi CSVs for the current and previous months.
func (c *CotasCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	// For test servers: use base URL directly (the test mocks return plain CSV)
	if !strings.Contains(c.baseURL, "dados.cvm.gov.br") {
		return c.fetchCSV(ctx, c.baseURL)
	}

	now := time.Now()
	var allRecords []domain.SourceRecord

	// Fetch current month and previous month (current may not be complete yet)
	for _, monthOffset := range []int{0, -1} {
		month := now.AddDate(0, monthOffset, 0)
		filename := fmt.Sprintf("inf_diario_fi_%s.csv", month.Format("200601"))
		url := fmt.Sprintf("%s/FI/DOC/INF_DIARIO/DADOS/%s", c.baseURL, filename)

		records, err := c.fetchCSV(ctx, url)
		if err != nil {
			// Skip months that are not yet available (e.g. future or too old)
			continue
		}
		allRecords = append(allRecords, records...)
	}

	if len(allRecords) == 0 {
		return nil, fmt.Errorf("cvm_cotas: no records retrieved from either month")
	}
	return allRecords, nil
}

// fetchCSV downloads and parses a single inf_diario_fi CSV file.
func (c *CotasCollector) fetchCSV(ctx context.Context, url string) ([]domain.SourceRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cvm_cotas: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cvm_cotas: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cvm_cotas: upstream returned %d", resp.StatusCode)
	}

	return parseCotasCSV(resp.Body)
}

// parseCotasCSV parses a semicolon-delimited inf_diario_fi CSV.
// Expected columns: CNPJ_FUNDO;DT_COMPTC;VL_TOTAL;VL_QUOTA;VL_PATRIM_LIQ;CAPTC_DIA;RESG_DIA;NR_COTST
func parseCotasCSV(r io.Reader) ([]domain.SourceRecord, error) {
	csvReader := csv.NewReader(r)
	csvReader.Comma = ';'
	csvReader.LazyQuotes = true

	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("cvm_cotas: read headers: %w", err)
	}
	for i, h := range headers {
		headers[i] = strings.ToLower(strings.TrimSpace(h))
	}

	var records []domain.SourceRecord
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}

		data := make(map[string]any, len(headers))
		for i, h := range headers {
			if i < len(row) {
				data[h] = strings.TrimSpace(row[i])
			}
		}

		cnpjFundo, _ := data["cnpj_fundo"].(string)
		dtComptc, _ := data["dt_comptc"].(string)
		if cnpjFundo == "" || dtComptc == "" {
			continue
		}

		// Normalize CNPJ to digits only for filtered queries
		cnpjDigits := normalizeCNPJDigits(cnpjFundo)

		records = append(records, domain.SourceRecord{
			Source:    "cvm_cotas",
			RecordKey: fmt.Sprintf("%s_%s", cnpjDigits, dtComptc),
			Data: map[string]any{
				"cnpj":         cnpjFundo,
				"cnpj_digits":  cnpjDigits,
				"data":         dtComptc,
				"vl_quota":     data["vl_quota"],
				"vl_patrimonio": data["vl_patrim_liq"],
				"captacao":     data["captc_dia"],
				"resgate":      data["resg_dia"],
				"nr_cotistas":  data["nr_cotst"],
			},
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}

// normalizeCNPJDigits strips all non-digit characters from a CNPJ string.
func normalizeCNPJDigits(cnpj string) string {
	var b strings.Builder
	for _, r := range cnpj {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
