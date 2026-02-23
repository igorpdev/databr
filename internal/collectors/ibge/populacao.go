package ibge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const (
	popTabela = "6579" // Estimativas de população
	popVar    = "9324" // população estimada
)

// PopulacaoCollector fetches population estimates from IBGE SIDRA table 6579.
type PopulacaoCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewPopulacaoCollector creates a population collector.
// baseURL should be the IBGE SIDRA base URL; if empty, the production URL is used.
func NewPopulacaoCollector(baseURL string) *PopulacaoCollector {
	if baseURL == "" {
		baseURL = sidraBase
	}
	return &PopulacaoCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *PopulacaoCollector) Source() string   { return "ibge_populacao" }
func (c *PopulacaoCollector) Schedule() string { return "0 8 1 * *" }

// Collect fetches the latest population estimates from IBGE SIDRA.
func (c *PopulacaoCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	// IMPORTANT: [all] must be percent-encoded as %5Ball%5D
	var url string
	if strings.Contains(c.baseURL, "servicodados.ibge.gov.br") {
		localidades := "%5Ball%5D" // url.PathEscape("[all]")
		url = fmt.Sprintf("%s/%s/periodos/-1/variaveis/%s?localidades=N1%s&view=flat",
			c.baseURL, popTabela, popVar, localidades)
	} else {
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ibge_populacao: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ibge_populacao: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ibge_populacao: upstream returned %d", resp.StatusCode)
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("ibge_populacao: decode: %w", err)
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("ibge_populacao: empty response")
	}

	records := make([]domain.SourceRecord, 0, len(raw))
	for _, entry := range raw {
		// SIDRA flat view fields
		locID, _ := entry["NC"].(string)   // localidade code
		locName, _ := entry["NN"].(string) // localidade name
		valor, _ := entry["V"].(string)    // population value
		periodo, _ := entry["D2C"].(string)

		if locID == "" {
			continue
		}

		records = append(records, domain.SourceRecord{
			Source:    "ibge_populacao",
			RecordKey: locID,
			Data: map[string]any{
				"localidade_id":   locID,
				"localidade_nome": locName,
				"populacao":       valor,
				"periodo":         periodo,
			},
			RawData:   entry,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
