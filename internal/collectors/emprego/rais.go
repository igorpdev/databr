// Package emprego implements collectors for Brazilian employment data sources (RAIS, CAGED).
package emprego

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const raisDefaultBaseURL = "https://dados.gov.br"

// raisRecord represents a single RAIS employment aggregate per sector.
type raisRecord struct {
	SectorCode string  `json:"sector_code"`
	SectorName string  `json:"sector_name"`
	Employees  int     `json:"employees"`
	AvgSalary  float64 `json:"avg_salary"`
	Admissions int     `json:"admissions"`
	Dismissals int     `json:"dismissals"`
	UF         string  `json:"uf"`
}

// RAISCollector fetches annual employment aggregates by sector from the RAIS data source.
type RAISCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewRAISCollector creates a new RAISCollector.
// baseURL overrides the production URL; pass "" to use the default.
func NewRAISCollector(baseURL string) *RAISCollector {
	if baseURL == "" {
		baseURL = raisDefaultBaseURL
	}
	return &RAISCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *RAISCollector) Source() string   { return "rais_emprego" }
func (c *RAISCollector) Schedule() string { return "0 3 1 * *" }

// Collect fetches RAIS employment data for the previous year.
func (c *RAISCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	year := time.Now().Year() - 1 // RAIS is released with ~1 year lag
	url := fmt.Sprintf("%s/api/rais?year=%d", c.baseURL, year)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("rais_emprego: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rais_emprego: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rais_emprego: upstream returned %d", resp.StatusCode)
	}

	var items []raisRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("rais_emprego: decode: %w", err)
	}

	now := time.Now().UTC()
	records := make([]domain.SourceRecord, 0, len(items))
	for _, item := range items {
		if item.SectorCode == "" {
			continue
		}

		recordKey := fmt.Sprintf("%d_%s", year, item.SectorCode)

		records = append(records, domain.SourceRecord{
			Source:    "rais_emprego",
			RecordKey: recordKey,
			Data: map[string]any{
				"year":        year,
				"sector_code": item.SectorCode,
				"sector_name": item.SectorName,
				"employees":   item.Employees,
				"avg_salary":  item.AvgSalary,
				"admissions":  item.Admissions,
				"dismissals":  item.Dismissals,
				"uf":          item.UF,
			},
			FetchedAt: now,
		})
	}

	return records, nil
}
