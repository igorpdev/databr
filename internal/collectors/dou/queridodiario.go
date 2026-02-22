// Package dou implements the Querido Diário collector (municipal official gazettes).
// API base: https://api.queridodiario.ok.org.br
// Note: the DOU (federal) has no public API — Querido Diário covers municipal gazettes.
package dou

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const qdBase = "https://api.queridodiario.ok.org.br"

// SearchParams holds search parameters for the Querido Diário API.
type SearchParams struct {
	Query       string // full-text search term
	UF          string // state code filter (ex: SP, RJ)
	TerritoryID string // IBGE municipality code
	Since       string // YYYY-MM-DD
	Until       string // YYYY-MM-DD
	Size        int    // max results, default 10
}

// QDCollector fetches municipal official gazette data from Querido Diário.
type QDCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewQDCollector creates a Querido Diário collector.
// Pass empty string for baseURL to use the production endpoint.
func NewQDCollector(baseURL string) *QDCollector {
	if baseURL == "" {
		baseURL = qdBase
	}
	return &QDCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *QDCollector) Source() string   { return "querido_diario" }
func (c *QDCollector) Schedule() string { return "@daily" }

// Collect is a no-op — QD is always queried on-demand.
func (c *QDCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	return nil, nil
}

// Search queries the Querido Diário API with the given parameters.
func (c *QDCollector) Search(ctx context.Context, params SearchParams) ([]domain.SourceRecord, error) {
	size := params.Size
	if size <= 0 {
		size = 10
	}

	q := url.Values{}
	if params.Query != "" {
		q.Set("querystring", params.Query)
	}
	if params.TerritoryID != "" {
		q.Set("territory_id", params.TerritoryID)
	}
	if params.Since != "" {
		q.Set("since", params.Since)
	}
	if params.Until != "" {
		q.Set("until", params.Until)
	}
	q.Set("size", fmt.Sprintf("%d", size))

	var reqURL string
	if strings.Contains(c.baseURL, "queridodiario.ok.org.br") {
		reqURL = fmt.Sprintf("%s/gazettes?%s", c.baseURL, q.Encode())
	} else {
		// Test server: use baseURL directly
		reqURL = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("querido_diario: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querido_diario: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("querido_diario: upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Gazettes []map[string]any `json:"gazettes"`
		Total    int              `json:"total_gazettes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("querido_diario: decode: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(raw.Gazettes))
	for i, g := range raw.Gazettes {
		records = append(records, domain.SourceRecord{
			Source:    "querido_diario",
			RecordKey: fmt.Sprintf("%s_%d", params.Query, i),
			Data:      g,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
