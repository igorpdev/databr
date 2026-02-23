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

// ComexImportCollector fetches import data from the ComexStat API (MDIC).
type ComexImportCollector struct {
	baseURL    string
	httpClient *http.Client
}

func NewComexImportCollector(baseURL string) *ComexImportCollector {
	if baseURL == "" {
		baseURL = comexBase
	}
	return &ComexImportCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *ComexImportCollector) Source() string   { return "comex_importacoes" }
func (c *ComexImportCollector) Schedule() string { return "0 21 7 * *" }

func (c *ComexImportCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	now := time.Now().UTC()
	prevMonth := now.AddDate(0, -1, 0)
	period := prevMonth.Format("200601")

	url := fmt.Sprintf("%s?flow=import&monthDetail=true&period.from=%s&period.to=%s",
		c.baseURL, period, period)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("comex_importacoes: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("comex_importacoes: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("comex_importacoes: API returned %d", resp.StatusCode)
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("comex_importacoes: decode: %w", err)
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("comex_importacoes: empty response for period %s", period)
	}

	records := make([]domain.SourceRecord, 0, len(raw))
	for i, entry := range raw {
		recordKey := fmt.Sprintf("import_%s_%d", period, i)
		records = append(records, domain.SourceRecord{
			Source:    "comex_importacoes",
			RecordKey: recordKey,
			Data:      entry,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
