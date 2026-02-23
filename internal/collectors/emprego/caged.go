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

const cagedDefaultBaseURL = "https://dados.gov.br"

// cagedRecord represents a single CAGED monthly job creation/destruction record.
type cagedRecord struct {
	Municipio     string `json:"municipio"`
	UF            string `json:"uf"`
	SectorCode    string `json:"sector_code"`
	SectorName    string `json:"sector_name"`
	Admissions    int    `json:"admissions"`
	Dismissals    int    `json:"dismissals"`
	NetBalance    int    `json:"net_balance"`
	SalarioMedio  float64 `json:"salario_medio"`
}

// CAGEDCollector fetches monthly job creation/destruction data from CAGED.
type CAGEDCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewCAGEDCollector creates a new CAGEDCollector.
// baseURL overrides the production URL; pass "" to use the default.
func NewCAGEDCollector(baseURL string) *CAGEDCollector {
	if baseURL == "" {
		baseURL = cagedDefaultBaseURL
	}
	return &CAGEDCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *CAGEDCollector) Source() string   { return "caged_emprego" }
func (c *CAGEDCollector) Schedule() string { return "0 3 15 * *" }

// Collect fetches CAGED data for the previous month.
func (c *CAGEDCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	// CAGED data is released with ~1 month lag.
	prevMonth := time.Now().AddDate(0, -1, 0)
	monthStr := prevMonth.Format("200601") // YYYYMM
	url := fmt.Sprintf("%s/api/caged?month=%s", c.baseURL, monthStr)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("caged_emprego: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("caged_emprego: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caged_emprego: upstream returned %d", resp.StatusCode)
	}

	var items []cagedRecord
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("caged_emprego: decode: %w", err)
	}

	now := time.Now().UTC()
	records := make([]domain.SourceRecord, 0, len(items))
	for i, item := range items {
		recordKey := fmt.Sprintf("caged_%s_%d", monthStr, i)

		records = append(records, domain.SourceRecord{
			Source:    "caged_emprego",
			RecordKey: recordKey,
			Data: map[string]any{
				"month":        monthStr,
				"municipio":    item.Municipio,
				"uf":           item.UF,
				"sector_code":  item.SectorCode,
				"sector_name":  item.SectorName,
				"admissions":   item.Admissions,
				"dismissals":   item.Dismissals,
				"net_balance":  item.NetBalance,
				"salario_medio": item.SalarioMedio,
			},
			FetchedAt: now,
		})
	}

	return records, nil
}
