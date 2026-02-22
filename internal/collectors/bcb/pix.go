package bcb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const pixBase = "https://olinda.bcb.gov.br/olinda/servico/Pix_DadosAbertos/versao/v1/odata"

// PIXCollector fetches PIX transaction statistics from BCB OLINDA.
type PIXCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewPIXCollector creates a PIX stats collector.
// Pass empty string for baseURL to use the production BCB OLINDA endpoint.
func NewPIXCollector(baseURL string) *PIXCollector {
	if baseURL == "" {
		baseURL = pixBase
	}
	return &PIXCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *PIXCollector) Source() string   { return "bcb_pix" }
func (c *PIXCollector) Schedule() string { return "@monthly" }

func (c *PIXCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "olinda.bcb.gov.br") {
		url = fmt.Sprintf("%s/EstatisticasTransacoesPix?$format=json&$top=12", c.baseURL)
	} else {
		// test server: use baseURL directly
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bcb_pix: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bcb_pix: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bcb_pix: upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Value []map[string]any `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bcb_pix: decode: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(raw.Value))
	for _, entry := range raw.Value {
		anoMes, _ := entry["AnoMes"].(string)
		if anoMes == "" {
			continue
		}
		records = append(records, domain.SourceRecord{
			Source:    "bcb_pix",
			RecordKey: anoMes,
			Data: map[string]any{
				"ano_mes":          anoMes,
				"qtd_transacoes":   entry["QtdTransacoes"],
				"valor_transacoes": entry["ValorTransacoes"],
			},
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
