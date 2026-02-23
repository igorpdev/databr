// Package comex implements collectors for Brazilian foreign trade (comércio exterior) data.
package comex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const comexBase = "https://api.comexstat.mdic.gov.br/general"

// ComexStatCollector fetches export data from the ComexStat API (MDIC).
type ComexStatCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewComexStatCollector creates a ComexStat collector.
// baseURL should be the ComexStat API base URL; if empty, the production URL is used.
func NewComexStatCollector(baseURL string) *ComexStatCollector {
	if baseURL == "" {
		baseURL = comexBase
	}
	return &ComexStatCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *ComexStatCollector) Source() string   { return "comex_exportacoes" }
func (c *ComexStatCollector) Schedule() string { return "0 8 1 * *" }

// Collect fetches export data for the previous month.
func (c *ComexStatCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	now := time.Now().UTC()
	prevMonth := now.AddDate(0, -1, 0)
	period := prevMonth.Format("200601") // YYYYMM

	url := fmt.Sprintf("%s?flow=export&monthDetail=true&period.from=%s&period.to=%s",
		c.baseURL, period, period)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("comex_exportacoes: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("comex_exportacoes: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("comex_exportacoes: API returned %d", resp.StatusCode)
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("comex_exportacoes: decode: %w", err)
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("comex_exportacoes: empty response for period %s", period)
	}

	records := make([]domain.SourceRecord, 0, len(raw))
	for i, entry := range raw {
		recordKey := fmt.Sprintf("export_%s_%d", period, i)
		records = append(records, domain.SourceRecord{
			Source:    "comex_exportacoes",
			RecordKey: recordKey,
			Data:      entry,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
