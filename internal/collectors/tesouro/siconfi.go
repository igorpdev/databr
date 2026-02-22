// Package tesouro implements collectors for Tesouro Nacional (SICONFI).
package tesouro

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

// siconfiBase is the production SICONFI API base URL.
// The path ending in "tt/" is correct — the double slash "tt//" in FetchRREO
// is intentional ORDS routing and must not be removed.
const siconfiBase = "https://apidatalake.tesouro.gov.br/ords/siconfi/tt/"

// SICONFICollector fetches fiscal data from SICONFI (Tesouro Nacional).
type SICONFICollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewSICONFICollector creates a SICONFI collector.
// Pass empty string for baseURL to use the production SICONFI endpoint.
func NewSICONFICollector(baseURL string) *SICONFICollector {
	if baseURL == "" {
		baseURL = siconfiBase
	}
	return &SICONFICollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *SICONFICollector) Source() string   { return "tesouro_siconfi" }
func (c *SICONFICollector) Schedule() string { return "@monthly" }

// Collect is a no-op — SICONFI is fetched on-demand per UF/ano/período.
func (c *SICONFICollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	return nil, nil
}

// FetchRREO fetches the RREO (Relatório Resumido da Execução Orçamentária)
// for a given UF, exercise year and period number.
// Note: when using the production URL, the double slash (tt//) is intentional ORDS routing.
func (c *SICONFICollector) FetchRREO(ctx context.Context, uf string, ano, periodo int) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "apidatalake.tesouro.gov.br") {
		// Double slash after "tt/" is intentional — required by ORDS routing
		url = fmt.Sprintf(
			"%s//rreo?an_exercicio=%d&nr_periodo=%d&co_tipo_demonstrativo=RREO&no_uf=%s",
			c.baseURL, ano, periodo, uf,
		)
	} else {
		// Test server: use baseURL directly
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("siconfi: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("siconfi: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("siconfi: upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("siconfi: decode: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(raw.Items))
	for _, item := range raw.Items {
		ente, _ := item["ente"].(string)
		if ente == "" {
			continue
		}
		key := fmt.Sprintf("%s_%d_%d", uf, ano, periodo)
		records = append(records, domain.SourceRecord{
			Source:    "tesouro_siconfi",
			RecordKey: key,
			Data:      item,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
