// Package transporte implements collectors for Brazilian transportation data sources.
// PRF collector fetches road accident data from the Polícia Rodoviária Federal open data portal.
package transporte

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const (
	prfDefaultBaseURL = "https://dados.prf.gov.br/dados"
	prfMaxRecords     = 1000
)

// prfColumns are the CSV column names from the PRF datatran export.
// Order matches the semicolon-delimited CSV header.
var prfColumns = []string{
	"id", "data_inversa", "dia_semana", "horario", "uf", "br", "km",
	"municipio", "causa_acidente", "tipo_acidente", "classificacao_acidente",
	"fase_dia", "sentido_via", "condicao_metereologica", "tipo_pista",
	"tracado_via", "uso_solo", "pessoas", "mortos", "feridos_leves",
	"feridos_graves", "ilesos", "ignorados", "feridos", "veiculos",
}

// PRFCollector fetches road accident data from PRF (Polícia Rodoviária Federal).
// The data is a semicolon-delimited CSV encoded in ISO-8859-1.
type PRFCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewPRFCollector creates a new PRFCollector.
// baseURL overrides the production URL; pass "" to use the default.
func NewPRFCollector(baseURL string) *PRFCollector {
	if baseURL == "" {
		baseURL = prfDefaultBaseURL
	}
	return &PRFCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *PRFCollector) Source() string   { return "prf_acidentes" }
func (c *PRFCollector) Schedule() string { return "0 8 1 * *" }

// Collect downloads the CSV for the current year and returns up to 1000 SourceRecords.
func (c *PRFCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	year := time.Now().Year()
	url := fmt.Sprintf("%s/datatran%d.csv", c.baseURL, year)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("prf_acidentes: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prf_acidentes: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prf_acidentes: upstream returned %d", resp.StatusCode)
	}

	// Transcode from ISO-8859-1 to UTF-8.
	utf8Body := transform.NewReader(resp.Body, charmap.ISO8859_1.NewDecoder())

	r := csv.NewReader(utf8Body)
	r.Comma = ';'
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	// Read header row.
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("prf_acidentes: read header: %w", err)
	}

	// Build column index from actual header.
	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	// Verify "id" column exists.
	idIdx, hasID := colIdx["id"]
	if !hasID {
		return nil, fmt.Errorf("prf_acidentes: 'id' column not found in header")
	}

	now := time.Now().UTC()
	var records []domain.SourceRecord

	for len(records) < prfMaxRecords {
		row, err := r.Read()
		if err != nil {
			if isEOF(err) {
				break
			}
			// Skip malformed rows.
			continue
		}

		if idIdx >= len(row) {
			continue
		}
		id := strings.TrimSpace(row[idIdx])
		if id == "" {
			continue
		}

		data := make(map[string]any, len(prfColumns))
		for _, col := range prfColumns {
			if idx, ok := colIdx[col]; ok && idx < len(row) {
				data[col] = strings.TrimSpace(row[idx])
			}
		}

		records = append(records, domain.SourceRecord{
			Source:    "prf_acidentes",
			RecordKey: id,
			Data:      data,
			FetchedAt: now,
		})
	}

	return records, nil
}
