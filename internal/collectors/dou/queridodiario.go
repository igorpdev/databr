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

// buildURL constructs a full URL by appending path and query to the base URL.
func (c *QDCollector) buildURL(path string, q url.Values) string {
	base := c.baseURL
	u := base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
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

	reqURL := c.buildURL("/gazettes", q)

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

// ListCities fetches municipalities with indexed official gazettes (level 3).
func (c *QDCollector) ListCities(ctx context.Context) ([]domain.SourceRecord, error) {
	q := url.Values{}
	q.Set("levels", "3")
	reqURL := c.buildURL("/cities", q)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("querido_diario: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querido_diario: fetch cities: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("querido_diario: cities upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Cities []map[string]any `json:"cities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("querido_diario: decode cities: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(raw.Cities))
	for i, city := range raw.Cities {
		tid, _ := city["territory_id"].(string)
		if tid == "" {
			tid = fmt.Sprintf("city_%d", i)
		}
		records = append(records, domain.SourceRecord{
			Source:    "querido_diario",
			RecordKey: tid,
			Data:      city,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}

// ListThemes fetches the list of automatic classification themes.
func (c *QDCollector) ListThemes(ctx context.Context) ([]string, error) {
	reqURL := c.buildURL("/gazettes/by_theme/themes/", nil)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("querido_diario: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querido_diario: fetch themes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("querido_diario: themes upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Themes []string `json:"themes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("querido_diario: decode themes: %w", err)
	}

	return raw.Themes, nil
}

// SearchByTheme searches official gazettes by classified theme.
// The theme name is URL-encoded in the path. Supports same query params as Search.
func (c *QDCollector) SearchByTheme(ctx context.Context, theme string, params SearchParams) ([]domain.SourceRecord, error) {
	size := params.Size
	if size <= 0 {
		size = 10
	}

	q := url.Values{}
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

	encodedTheme := url.PathEscape(theme)
	reqURL := c.buildURL("/gazettes/by_theme/"+encodedTheme, q)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("querido_diario: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querido_diario: fetch theme search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("querido_diario: theme search upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Excerpts []map[string]any `json:"excerpts"`
		Total    int              `json:"total_excerpts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("querido_diario: decode theme search: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(raw.Excerpts))
	for i, ex := range raw.Excerpts {
		records = append(records, domain.SourceRecord{
			Source:    "querido_diario",
			RecordKey: fmt.Sprintf("theme_%s_%d", theme, i),
			Data:      ex,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
