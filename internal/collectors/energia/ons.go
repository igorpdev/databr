package energia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const onsBase = "https://dados.ons.org.br/api/3/action"

// ONSCollector fetches electricity generation data from the ONS (Operador Nacional do Sistema) CKAN API.
type ONSCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewONSCollector creates an ONS collector.
// baseURL should be the ONS CKAN API base URL; if empty, the production URL is used.
func NewONSCollector(baseURL string) *ONSCollector {
	if baseURL == "" {
		baseURL = onsBase
	}
	return &ONSCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *ONSCollector) Source() string   { return "ons_geracao" }
func (c *ONSCollector) Schedule() string { return "0 9 * * *" }

// Collect fetches electricity generation data from ONS.
// It queries the CKAN datastore_search endpoint for the generation dataset.
func (c *ONSCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	url := fmt.Sprintf("%s/datastore_search?resource_id=geracao_usina_2_ho&limit=100", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ons_geracao: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ons_geracao: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ons_geracao: API returned %d", resp.StatusCode)
	}

	var envelope struct {
		Success bool `json:"success"`
		Result  struct {
			Records []map[string]any `json:"records"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("ons_geracao: decode: %w", err)
	}

	if !envelope.Success {
		return nil, fmt.Errorf("ons_geracao: API returned success=false")
	}

	if len(envelope.Result.Records) == 0 {
		return nil, fmt.Errorf("ons_geracao: empty response")
	}

	records := make([]domain.SourceRecord, 0, len(envelope.Result.Records))
	for _, entry := range envelope.Result.Records {
		subsistema, _ := entry["nom_subsistema"].(string)
		data, _ := entry["din_instante"].(string)

		recordKey := fmt.Sprintf("%s_%s", subsistema, data)
		if subsistema == "" && data == "" {
			// Use index-based key as fallback
			recordKey = fmt.Sprintf("record_%d", len(records))
		}

		records = append(records, domain.SourceRecord{
			Source:    "ons_geracao",
			RecordKey: recordKey,
			Data:      entry,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
