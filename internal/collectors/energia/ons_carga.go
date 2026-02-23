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

// ONSCargaCollector fetches electricity load/demand data from the ONS CKAN API.
type ONSCargaCollector struct {
	baseURL    string
	httpClient *http.Client
}

func NewONSCargaCollector(baseURL string) *ONSCargaCollector {
	if baseURL == "" {
		baseURL = onsBase
	}
	return &ONSCargaCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *ONSCargaCollector) Source() string   { return "ons_carga" }
func (c *ONSCargaCollector) Schedule() string { return "0 15 * * *" }

func (c *ONSCargaCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	url := fmt.Sprintf("%s/datastore_search?resource_id=carga_energia_2_ho&limit=100", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ons_carga: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ons_carga: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ons_carga: API returned %d", resp.StatusCode)
	}

	var envelope struct {
		Success bool `json:"success"`
		Result  struct {
			Records []map[string]any `json:"records"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("ons_carga: decode: %w", err)
	}

	if !envelope.Success {
		return nil, fmt.Errorf("ons_carga: API returned success=false")
	}

	if len(envelope.Result.Records) == 0 {
		return nil, fmt.Errorf("ons_carga: empty response")
	}

	records := make([]domain.SourceRecord, 0, len(envelope.Result.Records))
	for _, entry := range envelope.Result.Records {
		subsistema, _ := entry["nom_subsistema"].(string)
		data, _ := entry["din_instante"].(string)

		recordKey := fmt.Sprintf("%s_%s", subsistema, data)
		if subsistema == "" && data == "" {
			recordKey = fmt.Sprintf("record_%d", len(records))
		}

		records = append(records, domain.SourceRecord{
			Source:    "ons_carga",
			RecordKey: recordKey,
			Data:      entry,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
