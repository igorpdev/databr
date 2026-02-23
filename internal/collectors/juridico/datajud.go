// Package juridico implements the DataJud CNJ collector for judicial processes.
// API: https://api-publica.datajud.cnj.jus.br/
// Auth: Authorization: APIKey {key} — free public key from https://datajud-wiki.cnj.jus.br
package juridico

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const datajudBase = "https://api-publica.datajud.cnj.jus.br"

// tribunais to search across — 23 major courts by case volume.
// Each is a separate Elasticsearch index in DataJud.
var tribunais = []string{
	"api_publica_tjsp", "api_publica_tjrj", "api_publica_tjmg",
	"api_publica_tjrs", "api_publica_tjpr", "api_publica_tjba",
	"api_publica_tjsc", "api_publica_tjpe", "api_publica_tjgo",
	"api_publica_tjce",
	"api_publica_trf1", "api_publica_trf2", "api_publica_trf3",
	"api_publica_trf4", "api_publica_trf5", "api_publica_trf6",
	"api_publica_trt1", "api_publica_trt2", "api_publica_trt3",
	"api_publica_trt4", "api_publica_trt15",
	"api_publica_stj", "api_publica_tst",
}

// DataJudCollector fetches judicial process data from DataJud CNJ.
type DataJudCollector struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewDataJudCollector creates a DataJud collector.
// apiKey should be read from DATAJUD_API_KEY env var.
func NewDataJudCollector(baseURL, apiKey string) *DataJudCollector {
	if baseURL == "" {
		baseURL = datajudBase
	}
	return &DataJudCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *DataJudCollector) Source() string   { return "datajud_cnj" }
func (c *DataJudCollector) Schedule() string { return "@daily" }

// Collect is a no-op — DataJud is queried on-demand by document.
func (c *DataJudCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	return nil, nil
}

// Search queries DataJud for processes linked to the given CPF (11 digits) or CNPJ (14 digits).
// Searches across the configured list of major tribunals, stopping at 50 results.
func (c *DataJudCollector) Search(ctx context.Context, documento string) ([]domain.SourceRecord, error) {
	// Normalize: keep digits only
	normalized := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, documento)

	fieldName := "CPF"
	if len(normalized) == 14 {
		fieldName = "CNPJ"
	}

	var allRecords []domain.SourceRecord
	for _, tribunal := range tribunais {
		records, err := c.searchTribunal(ctx, tribunal, fieldName, normalized)
		if err != nil {
			// Non-fatal: one tribunal failing shouldn't block results from others
			continue
		}
		allRecords = append(allRecords, records...)
		if len(allRecords) >= 50 {
			break
		}
	}
	return allRecords, nil
}

func (c *DataJudCollector) searchTribunal(ctx context.Context, tribunal, fieldName, doc string) ([]domain.SourceRecord, error) {
	body := map[string]any{
		"size": 5,
		"query": map[string]any{
			"nested": map[string]any{
				"path": "partes",
				"query": map[string]any{
					"match": map[string]any{
						"partes." + fieldName: doc,
					},
				},
			},
		},
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("datajud: marshal body: %w", err)
	}

	var reqURL string
	if strings.Contains(c.baseURL, "datajud.cnj.jus.br") {
		reqURL = fmt.Sprintf("%s/%s/_search", c.baseURL, tribunal)
	} else {
		// Test server: use baseURL directly
		reqURL = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "APIKey "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("datajud: %s returned %d", tribunal, resp.StatusCode)
	}

	var raw struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	records := make([]domain.SourceRecord, 0, len(raw.Hits.Hits))
	for i, hit := range raw.Hits.Hits {
		numProcesso, _ := hit.Source["numeroProcesso"].(string)
		if numProcesso == "" {
			numProcesso = fmt.Sprintf("%s_%d", tribunal, i)
		}
		records = append(records, domain.SourceRecord{
			Source:    "datajud_cnj",
			RecordKey: numProcesso,
			Data:      hit.Source,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
