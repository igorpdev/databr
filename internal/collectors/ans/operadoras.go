// Package ans implements collectors for ANS (Agencia Nacional de Saude Suplementar) data.
package ans

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const (
	ansDefaultURL = "https://dadosabertos.ans.gov.br/FTP/PDA/operadoras_de_plano_de_saude_ativas/Relatorio_cadop.csv"
)

// OperadorasCollector fetches the ANS open data CSV of active health plan operators.
type OperadorasCollector struct {
	csvURL     string
	httpClient *http.Client
}

// NewOperadorasCollector creates a new OperadorasCollector.
// csvURL overrides the production ANS CSV URL; pass "" to use the default.
func NewOperadorasCollector(csvURL string) *OperadorasCollector {
	if csvURL == "" {
		csvURL = ansDefaultURL
	}
	return &OperadorasCollector{
		csvURL: csvURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *OperadorasCollector) Source() string   { return "ans_operadoras" }
func (c *OperadorasCollector) Schedule() string { return "0 9 * * 1" }

// normalizeKey converts a CSV header name to a lowercase underscore key.
// Example: "Registro_ANS" -> "registro_ans", "Razao_Social" -> "razao_social".
func normalizeKey(header string) string {
	return strings.ToLower(strings.TrimSpace(header))
}

// Collect downloads the ANS operadoras CSV, parses it, and returns one
// SourceRecord per active operator. The file is UTF-8 encoded and uses
// semicolons as field separators.
func (c *OperadorasCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.csvURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ans_operadoras: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ans_operadoras: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ans_operadoras: upstream returned %d", resp.StatusCode)
	}

	r := csv.NewReader(resp.Body)
	r.Comma = ';'
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	// Read header row dynamically.
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("ans_operadoras: read header: %w", err)
	}

	// Build normalized key names and find the Registro_ANS column index.
	keys := make([]string, len(header))
	registroIdx := -1
	for i, h := range header {
		keys[i] = normalizeKey(h)
		if keys[i] == "registro_ans" {
			registroIdx = i
		}
	}
	if registroIdx == -1 {
		return nil, fmt.Errorf("ans_operadoras: column Registro_ANS not found in header")
	}

	now := time.Now().UTC()
	var records []domain.SourceRecord

	for {
		row, err := r.Read()
		if err != nil {
			if isEOF(err) {
				break
			}
			// Skip malformed rows without aborting.
			continue
		}

		// Skip rows that don't have enough columns.
		if len(row) < len(keys) {
			continue
		}

		registroANS := strings.TrimSpace(row[registroIdx])
		if registroANS == "" {
			continue
		}

		data := make(map[string]any, len(keys))
		for i, key := range keys {
			data[key] = strings.TrimSpace(row[i])
		}

		records = append(records, domain.SourceRecord{
			Source:    "ans_operadoras",
			RecordKey: registroANS,
			Data:      data,
			FetchedAt: now,
		})
	}

	return records, nil
}

// isEOF reports whether err signals end of file.
func isEOF(err error) bool {
	return errors.Is(err, io.EOF)
}
