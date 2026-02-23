package b3

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const b3IndicesBase = "https://sistemaswebb3-listados.b3.com.br/indexPage"

// IndicesCollector fetches the IBOVESPA index composition from B3.
type IndicesCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewIndicesCollector creates a B3 indices collector.
// baseURL should be the B3 indices API base URL; if empty, the production URL is used.
func NewIndicesCollector(baseURL string) *IndicesCollector {
	if baseURL == "" {
		baseURL = b3IndicesBase
	}
	return &IndicesCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *IndicesCollector) Source() string   { return "b3_ibovespa" }
func (c *IndicesCollector) Schedule() string { return "0 18 * * 1-5" }

// Collect fetches the IBOVESPA index composition for the current day.
func (c *IndicesCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	url := fmt.Sprintf("%s/day/IBOV?language=pt-br", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("b3_ibovespa: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("b3_ibovespa: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("b3_ibovespa: API returned %d", resp.StatusCode)
	}

	var envelope struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("b3_ibovespa: decode: %w", err)
	}

	if len(envelope.Results) == 0 {
		return nil, fmt.Errorf("b3_ibovespa: empty response")
	}

	today := LastBusinessDay(time.Now()).Format("20060102")
	recordKey := fmt.Sprintf("IBOV_%s", today)

	records := []domain.SourceRecord{
		{
			Source:    "b3_ibovespa",
			RecordKey: recordKey,
			Data: map[string]any{
				"indice":      "IBOV",
				"data":        today,
				"composicao":  envelope.Results,
				"total_ativos": len(envelope.Results),
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	return records, nil
}
