package tcu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/databr/api/internal/domain"
)

// AcordaosCollector fetches TCU court decisions (acordaos).
type AcordaosCollector struct {
	httpClient *http.Client
	baseURL    string
}

// NewAcordaosCollector creates a new AcordaosCollector.
func NewAcordaosCollector(_ string) *AcordaosCollector {
	return &AcordaosCollector{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://dados-abertos.apps.tcu.gov.br/api/acordao/recupera-acordaos",
	}
}

// NewAcordaosCollectorWithURL creates a collector with a custom base URL (for testing).
func NewAcordaosCollectorWithURL(baseURL string) *AcordaosCollector {
	return &AcordaosCollector{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

func (c *AcordaosCollector) Source() string   { return "tcu_acordaos" }
func (c *AcordaosCollector) Schedule() string { return "0 13 * * 3,4" }

func (c *AcordaosCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	url := c.baseURL + "?inicio=0&quantidade=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("tcu_acordaos: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tcu_acordaos: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tcu_acordaos: upstream returned %d", resp.StatusCode)
	}

	var acordaos []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&acordaos); err != nil {
		return nil, fmt.Errorf("tcu_acordaos: decode: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(acordaos))
	now := time.Now().UTC()
	for _, ac := range acordaos {
		key := fmt.Sprintf("%v_%v_%v", ac["tipo"], ac["numero"], ac["anoAcordao"])
		records = append(records, domain.SourceRecord{
			Source:    "tcu_acordaos",
			RecordKey: key,
			Data:      ac,
			FetchedAt: now,
		})
	}
	return records, nil
}
