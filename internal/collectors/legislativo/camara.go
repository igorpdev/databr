// Package legislativo implements collectors for Brazilian legislative data sources
// (Câmara dos Deputados and Senado Federal).
package legislativo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const camaraBase = "https://dadosabertos.camara.leg.br/api/v2"

// CamaraCollector fetches the list of current deputies from the Câmara dos Deputados API.
type CamaraCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewCamaraCollector creates a Câmara collector.
// baseURL should be the Câmara API v2 base URL; if empty, the production URL is used.
func NewCamaraCollector(baseURL string) *CamaraCollector {
	if baseURL == "" {
		baseURL = camaraBase
	}
	return &CamaraCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CamaraCollector) Source() string   { return "camara_deputados" }
func (c *CamaraCollector) Schedule() string { return "0 7 * * *" }

// Collect fetches the list of deputies from the Câmara dos Deputados.
func (c *CamaraCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	url := fmt.Sprintf("%s/deputados?ordem=ASC&ordenarPor=nome&itens=100", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("camara_deputados: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("camara_deputados: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("camara_deputados: API returned %d", resp.StatusCode)
	}

	var envelope struct {
		Dados []map[string]any `json:"dados"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("camara_deputados: decode: %w", err)
	}

	if len(envelope.Dados) == 0 {
		return nil, fmt.Errorf("camara_deputados: empty response")
	}

	records := make([]domain.SourceRecord, 0, len(envelope.Dados))
	for _, dep := range envelope.Dados {
		id := fmt.Sprintf("%v", dep["id"])
		records = append(records, domain.SourceRecord{
			Source:    "camara_deputados",
			RecordKey: id,
			Data:      dep,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
