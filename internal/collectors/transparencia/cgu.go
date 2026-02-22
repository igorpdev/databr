// Package transparencia implements collectors for the Portal da Transparência (CGU).
// Authentication: header chave-api-dados is required.
// Rate limit: 90 req/min (06h-23h59) | 300 req/min (00h-05h59)
package transparencia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const transparenciaBase = "https://api.portaldatransparencia.gov.br/api-de-dados"

// CGUCollector fetches compliance data (CEIS, CNEP) from the Portal da Transparência.
type CGUCollector struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewCGUCollector creates a CGU collector.
// apiKey is from TRANSPARENCIA_API_KEY environment variable.
func NewCGUCollector(baseURL, apiKey string) *CGUCollector {
	if baseURL == "" {
		baseURL = transparenciaBase
	}
	return &CGUCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *CGUCollector) Source() string   { return "cgu_compliance" }
func (c *CGUCollector) Schedule() string { return "@daily" }

// Collect is intentionally a no-op — CGU data is fetched on-demand by CNPJ.
func (c *CGUCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	return nil, nil
}

// FetchByCNPJ fetches CEIS and CNEP data for a specific CNPJ.
// Returns a single record aggregating both lists.
func (c *CGUCollector) FetchByCNPJ(ctx context.Context, cnpjNum string) ([]domain.SourceRecord, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("cgu_compliance: TRANSPARENCIA_API_KEY is not set")
	}

	ceis, err := c.fetchList(ctx, "/ceis", cnpjNum)
	if err != nil {
		return nil, fmt.Errorf("cgu_compliance: CEIS fetch: %w", err)
	}

	cnep, err := c.fetchList(ctx, "/cnep", cnpjNum)
	if err != nil {
		return nil, fmt.Errorf("cgu_compliance: CNEP fetch: %w", err)
	}

	record := domain.SourceRecord{
		Source:    "cgu_compliance",
		RecordKey: cnpjNum,
		Data: map[string]any{
			"cnpj":      cnpjNum,
			"ceis":      ceis,
			"cnep":      cnep,
			"sanitized": len(ceis) == 0 && len(cnep) == 0,
		},
		FetchedAt: time.Now().UTC(),
	}

	return []domain.SourceRecord{record}, nil
}

// fetchList fetches a list endpoint (e.g. /ceis) filtered by CNPJ.
func (c *CGUCollector) fetchList(ctx context.Context, path, cnpjNum string) ([]any, error) {
	url := fmt.Sprintf("%s%s?cnpjSancionado=%s&pagina=1", c.baseURL, path, cnpjNum)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("chave-api-dados", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("cgu_compliance: invalid API key (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cgu_compliance: %s returned %d", path, resp.StatusCode)
	}

	// CGU returns {"data": [...]} or just [...]
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	if data, ok := raw["data"]; ok {
		if list, ok := data.([]any); ok {
			return list, nil
		}
	}
	return []any{}, nil
}
