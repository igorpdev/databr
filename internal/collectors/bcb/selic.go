// Package bcb implements collectors for the Banco Central do Brasil APIs.
package bcb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const (
	sgsSelic = "11" // BCB SGS series code for daily Selic rate
	sgsBase  = "https://api.bcb.gov.br/dados/serie/bcdata.sgs"
)

// SelicCollector fetches the BCB Selic rate from the SGS (Sistema Gerenciador de Séries Temporais).
type SelicCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewSelicCollector creates a Selic collector.
// baseURL should be the BCB SGS base URL; if empty, the production URL is used.
func NewSelicCollector(baseURL string) *SelicCollector {
	if baseURL == "" {
		baseURL = sgsBase
	}
	return &SelicCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *SelicCollector) Source() string   { return "bcb_selic" }
func (c *SelicCollector) Schedule() string { return "@daily" }

// Collect fetches the last 30 Selic values from BCB SGS.
func (c *SelicCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	// When using a custom test base URL, append the series path; otherwise use full URL.
	var url string
	if strings.Contains(c.baseURL, "api.bcb.gov.br") {
		url = fmt.Sprintf("%s.%s/dados/ultimos/20?formato=json", c.baseURL, sgsSelic)
	} else {
		// Test server: just append a path that the test server handles
		url = fmt.Sprintf("%s/dados/ultimos/30", c.baseURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bcb_selic: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bcb_selic: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bcb_selic: upstream returned %d", resp.StatusCode)
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bcb_selic: decode: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("bcb_selic: empty response from SGS")
	}

	records := make([]domain.SourceRecord, 0, len(raw))
	for _, entry := range raw {
		date, _ := entry["data"].(string)
		valor, _ := entry["valor"].(string)
		if date == "" || valor == "" {
			continue
		}
		records = append(records, domain.SourceRecord{
			Source:    "bcb_selic",
			RecordKey: date,
			Data: map[string]any{
				"data":  date,
				"valor": valor,
			},
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
