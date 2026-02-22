// Package cnpj implements the CNPJ data collector using minhareceita.org.
// This collector is on-demand only (no background schedule) because CNPJ
// data is requested per-query rather than pre-synced in bulk.
package cnpj

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/databr/api/internal/domain"
)

// Collector fetches CNPJ data from minhareceita.org (or a compatible endpoint).
type Collector struct {
	baseURL    string
	httpClient *http.Client
}

// NewCollector creates a new CNPJ collector.
// baseURL defaults to https://minhareceita.org if empty.
func NewCollector(baseURL string) *Collector {
	if baseURL == "" {
		baseURL = "https://minhareceita.org"
	}
	return &Collector{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Source returns the unique identifier for this collector.
func (c *Collector) Source() string { return "cnpj" }

// Schedule returns an empty string because CNPJ is on-demand, not scheduled.
func (c *Collector) Schedule() string { return "" }

// Collect is a no-op for CNPJ (on-demand only). Use FetchByCNPJ instead.
func (c *Collector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	return nil, nil
}

// FetchByCNPJ fetches and normalizes data for a single CNPJ.
// cnpjNum should be a 14-digit string (digits only).
func (c *Collector) FetchByCNPJ(ctx context.Context, cnpjNum string) ([]domain.SourceRecord, error) {
	cnpjNum = NormalizeCNPJ(cnpjNum)

	url := fmt.Sprintf("%s/%s", c.baseURL, cnpjNum)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cnpj: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cnpj: fetch %s: %w", cnpjNum, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("cnpj: %s not found (404)", cnpjNum)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cnpj: upstream returned %d for %s", resp.StatusCode, cnpjNum)
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("cnpj: decode response: %w", err)
	}

	record := domain.SourceRecord{
		Source:    "cnpj",
		RecordKey: cnpjNum,
		Data:      normalizeData(raw),
		RawData:   raw,
		FetchedAt: time.Now().UTC(),
	}

	return []domain.SourceRecord{record}, nil
}

// normalizeData extracts and renames the relevant fields from a minhareceita response.
func normalizeData(raw map[string]any) map[string]any {
	data := make(map[string]any, len(raw))
	for k, v := range raw {
		data[k] = v
	}
	return data
}

// NormalizeCNPJ strips all non-digit characters from a CNPJ string.
// "12.345.678/0001-95" → "12345678000195"
func NormalizeCNPJ(cnpj string) string {
	var b strings.Builder
	for _, r := range cnpj {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
