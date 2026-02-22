// Package cvm implements collectors for the CVM (Comissão de Valores Mobiliários).
// CVM data is served as CSV/ZIP file downloads (not JSON APIs).
package cvm

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const cvmBase = "https://dados.cvm.gov.br/dados"

// FundosCollector downloads the daily CVM fund informes (inf_diario_fi).
type FundosCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewFundosCollector creates a CVM fundos collector.
func NewFundosCollector(baseURL string) *FundosCollector {
	if baseURL == "" {
		baseURL = cvmBase
	}
	return &FundosCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *FundosCollector) Source() string   { return "cvm_fundos" }
func (c *FundosCollector) Schedule() string { return "@daily" }

// Collect downloads and parses the latest monthly CVM fund informe.
func (c *FundosCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	// For test servers: just call the base URL directly
	var url string
	if strings.Contains(c.baseURL, "dados.cvm.gov.br") {
		now := time.Now()
		// Use previous month (current month file may not be available yet)
		month := now.AddDate(0, -1, 0)
		filename := fmt.Sprintf("inf_diario_fi_%s.zip", month.Format("200601"))
		url = fmt.Sprintf("%s/FI/DOC/INF_DIARIO/DADOS/%s", c.baseURL, filename)
	} else {
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cvm_fundos: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cvm_fundos: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cvm_fundos: upstream returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cvm_fundos: read body: %w", err)
	}

	return parseZipCSV(body)
}

// parseZipCSV extracts and parses a semicolon-delimited CSV from a ZIP archive.
func parseZipCSV(zipData []byte) ([]domain.SourceRecord, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("cvm_fundos: open zip: %w", err)
	}

	var records []domain.SourceRecord
	for _, f := range r.File {
		if !strings.HasSuffix(f.Name, ".csv") && !strings.HasSuffix(f.Name, ".CSV") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("cvm_fundos: open %s: %w", f.Name, err)
		}
		rows, err := parseCSV(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		records = append(records, rows...)
	}
	return records, nil
}

func parseCSV(r io.Reader) ([]domain.SourceRecord, error) {
	csvReader := csv.NewReader(r)
	csvReader.Comma = ';'
	csvReader.LazyQuotes = true

	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("cvm_fundos: read headers: %w", err)
	}
	// Normalize headers to lowercase
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

		// Rename CSV columns to our normalized names
		normalized := map[string]any{
			"cnpj_fundo":      data["cnpj_fundo"],
			"data_competencia": data["dt_comptc"],
			"vl_total":        data["vl_total"],
			"vl_quota":        data["vl_quota"],
			"vl_patrim_liq":   data["vl_patrim_liq"],
			"captc_dia":       data["captc_dia"],
			"resg_dia":        data["resg_dia"],
			"nr_cotistas":     data["nr_cotst"],
		}

		cnpj, _ := data["cnpj_fundo"].(string)
		date, _ := data["dt_comptc"].(string)
		if cnpj == "" || date == "" {
			continue
		}

		records = append(records, domain.SourceRecord{
			Source:    "cvm_fundos",
			RecordKey: fmt.Sprintf("%s_%s", cnpj, date),
			Data:      normalized,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
