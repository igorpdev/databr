// Package educacao implements collectors for Brazilian education data sources.
package educacao

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const inepBase = "https://dadosabertos.mec.gov.br/api/3/action"

// INEPCollector fetches education indicators from the INEP/MEC open data API.
type INEPCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewINEPCollector creates an INEP collector.
// baseURL should be the INEP/MEC CKAN API base URL; if empty, the production URL is used.
func NewINEPCollector(baseURL string) *INEPCollector {
	if baseURL == "" {
		baseURL = inepBase
	}
	return &INEPCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *INEPCollector) Source() string   { return "inep_censo_escolar" }
func (c *INEPCollector) Schedule() string { return "0 8 1 2,9 *" }

// Collect fetches education indicator data from INEP.
// Uses the CKAN datastore_search endpoint to retrieve census data.
func (c *INEPCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	url := fmt.Sprintf("%s/datastore_search?resource_id=indicadores-educacionais&limit=100", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("inep_censo_escolar: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("inep_censo_escolar: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inep_censo_escolar: API returned %d", resp.StatusCode)
	}

	var envelope struct {
		Success bool `json:"success"`
		Result  struct {
			Records []map[string]any `json:"records"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("inep_censo_escolar: decode: %w", err)
	}

	if !envelope.Success {
		return nil, fmt.Errorf("inep_censo_escolar: API returned success=false")
	}

	if len(envelope.Result.Records) == 0 {
		return nil, fmt.Errorf("inep_censo_escolar: empty response")
	}

	records := make([]domain.SourceRecord, 0, len(envelope.Result.Records))
	for _, entry := range envelope.Result.Records {
		ano, _ := entry["ano"].(string)
		indicador, _ := entry["indicador"].(string)

		recordKey := fmt.Sprintf("%s_%s", ano, indicador)
		if ano == "" && indicador == "" {
			recordKey = fmt.Sprintf("record_%d", len(records))
		}

		records = append(records, domain.SourceRecord{
			Source:    "inep_censo_escolar",
			RecordKey: recordKey,
			Data:      entry,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
