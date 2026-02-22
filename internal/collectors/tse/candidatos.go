// Package tse implements collectors for the TSE (Tribunal Superior Eleitoral).
// Data is served via CDN as ZIP archives with semicolon-delimited CSV files.
// CDN base: https://cdn.tse.jus.br/estatistica/sead/odsele/
package tse

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

const tseCDN = "https://cdn.tse.jus.br/estatistica/sead/odsele"

// CandidatosCollector downloads TSE candidate data for a given election year.
type CandidatosCollector struct {
	baseURL    string
	ano        int
	httpClient *http.Client
}

// NewCandidatosCollector creates a TSE candidatos collector for the given year.
func NewCandidatosCollector(baseURL string) *CandidatosCollector {
	if baseURL == "" {
		baseURL = tseCDN
	}
	return &CandidatosCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		ano:        2024, // most recent election
		httpClient: &http.Client{Timeout: 120 * time.Second}, // large files
	}
}

func (c *CandidatosCollector) Source() string   { return "tse_candidatos" }
func (c *CandidatosCollector) Schedule() string { return "@yearly" }

// Collect downloads and parses TSE candidatos for the configured election year.
func (c *CandidatosCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "cdn.tse.jus.br") {
		url = fmt.Sprintf("%s/consulta_cand/consulta_cand_%d.zip", c.baseURL, c.ano)
	} else {
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("tse_candidatos: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tse_candidatos: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tse_candidatos: upstream returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tse_candidatos: read: %w", err)
	}

	return parseZipCandidatos(body)
}

func parseZipCandidatos(zipData []byte) ([]domain.SourceRecord, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("tse_candidatos: open zip: %w", err)
	}

	var allRecords []domain.SourceRecord
	for _, f := range r.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		records, err := parseTSECSV(rc)
		rc.Close()
		if err != nil {
			continue
		}
		allRecords = append(allRecords, records...)
	}
	return allRecords, nil
}

func parseTSECSV(r io.Reader) ([]domain.SourceRecord, error) {
	csvReader := csv.NewReader(r)
	csvReader.Comma = ';'
	csvReader.LazyQuotes = true

	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("tse_candidatos: read headers: %w", err)
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
			continue
		}

		data := make(map[string]any, len(headers))
		for i, h := range headers {
			if i < len(row) {
				data[h] = strings.TrimSpace(row[i])
			}
		}

		sq, _ := data["sq_candidato"].(string)
		if sq == "" {
			continue
		}

		records = append(records, domain.SourceRecord{
			Source:    "tse_candidatos",
			RecordKey: sq,
			Data:      data,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
