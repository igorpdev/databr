// Package ipea implements collectors for IPEAData (Instituto de Pesquisa Economica Aplicada).
package ipea

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const defaultBaseURL = "http://ipeadata.gov.br/api/odata4/"

// macroSeries defines the pre-selected macro-economic series to collect.
var macroSeries = []struct {
	Code string
	Name string
}{
	{"BM12_TJOVER12", "SELIC overnight"},
	{"PRECOS12_IPCAG12", "IPCA acumulado"},
	{"SCN104_PIBPMG104", "PIB trimestral"},
	{"BM12_CRLIN12", "Credito livre"},
	{"GAC12_TCAMBIO12", "Cambio medio"},
	{"PNADC12_TDGA12", "Taxa desemprego"},
}

// odataResponse mirrors the OData v4 JSON envelope from IPEAData.
type odataResponse struct {
	Value []odataValue `json:"value"`
}

// odataValue represents a single data point in the IPEAData OData response.
type odataValue struct {
	SerCodigo string  `json:"SERCODIGO"`
	ValData   string  `json:"VALDATA"`
	ValValor  float64 `json:"VALVALOR"`
}

// SeriesCollector fetches macro-economic time series from IPEAData's OData v4 API.
type SeriesCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewSeriesCollector creates a new SeriesCollector.
// baseURL overrides the production IPEAData API URL; pass "" to use the default.
func NewSeriesCollector(baseURL string) *SeriesCollector {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	// Ensure trailing slash for URL joining.
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &SeriesCollector{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *SeriesCollector) Source() string   { return "ipea_series" }
func (c *SeriesCollector) Schedule() string { return "@daily" }

// Collect fetches the last 12 data points for each pre-defined macro series.
// If a single series fetch fails, it logs a warning and continues to the next.
func (c *SeriesCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	now := time.Now().UTC()
	var records []domain.SourceRecord

	for _, s := range macroSeries {
		vals, err := c.fetchSeries(ctx, s.Code)
		if err != nil {
			slog.Warn("ipea_series: series fetch failed", "code", s.Code, "name", s.Name, "error", err)
			continue
		}

		for _, v := range vals {
			dateStr := extractDate(v.ValData)
			records = append(records, domain.SourceRecord{
				Source:    "ipea_series",
				RecordKey: fmt.Sprintf("%s_%s", s.Code, dateStr),
				Data: map[string]any{
					"codigo": s.Code,
					"nome":   s.Name,
					"data":   dateStr,
					"valor":  v.ValValor,
				},
				FetchedAt: now,
			})
		}
	}

	return records, nil
}

// fetchSeries retrieves the last 12 data points for a given series code.
func (c *SeriesCollector) fetchSeries(ctx context.Context, code string) ([]odataValue, error) {
	url := fmt.Sprintf("%sValoresSerie(SERCODIGO='%s')?$top=12&$orderby=VALDATA%%20desc", c.baseURL, code)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	// CRITICAL: IPEAData does NOT support $format=json — must use Accept header.
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	var odata odataResponse
	if err := json.NewDecoder(resp.Body).Decode(&odata); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	return odata.Value, nil
}

// extractDate parses a VALDATA string (e.g. "2026-02-01T00:00:00-03:00") and
// returns the YYYY-MM-DD portion. Falls back to the raw string on parse failure.
func extractDate(valdata string) string {
	// Try RFC3339 first (handles timezone offsets).
	if t, err := time.Parse(time.RFC3339, valdata); err == nil {
		return t.Format("2006-01-02")
	}
	// Fallback: extract the first 10 chars if they look like a date.
	if len(valdata) >= 10 {
		return valdata[:10]
	}
	return valdata
}
