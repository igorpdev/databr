// Package ibge implements collectors for IBGE APIs (SIDRA).
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
	sidraBase  = "https://servicodados.ibge.gov.br/api/v3/agregados"
	ipcaTabela = "1737" // IPCA mensal
	ipcaVar    = "63"   // variação mensal (%)
)

// IPCACollector fetches the IPCA (inflation index) from IBGE SIDRA.
type IPCACollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewIPCACollector creates an IPCA collector.
func NewIPCACollector(baseURL string) *IPCACollector {
	if baseURL == "" {
		baseURL = sidraBase
	}
	return &IPCACollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *IPCACollector) Source() string   { return "ibge_ipca" }
func (c *IPCACollector) Schedule() string { return "0 8 * * *" }

// Collect fetches the last 12 months of IPCA data from SIDRA.
func (c *IPCACollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	// IMPORTANT: [all] in localidades must be percent-encoded as %5Ball%5D
	// Do NOT use http.DefaultClient auto-escape — it corrupts the URL.
	// Reference: docs/plans/2026-02-22-contratos-verificados.md
	var url string
	if strings.Contains(c.baseURL, "servicodados.ibge.gov.br") {
		localidades := "%5Ball%5D" // url.PathEscape("[all]")
		url = fmt.Sprintf("%s/%s/periodos/-12/variaveis/%s?localidades=N1%s&view=flat",
			c.baseURL, ipcaTabela, ipcaVar, localidades)
	} else {
		url = c.baseURL
	}

	return c.fetch(ctx, url, "ibge_ipca")
}

func (c *IPCACollector) fetch(ctx context.Context, url, source string) ([]domain.SourceRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: build request: %w", source, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: fetch: %w", source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: upstream returned %d", source, resp.StatusCode)
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", source, err)
	}

	records := make([]domain.SourceRecord, 0, len(raw))
	for _, entry := range raw {
		period, _ := entry["D2C"].(string) // e.g. "202601"
		if period == "" {
			continue
		}
		periodName, _ := entry["D2N"].(string) // e.g. "janeiro 2026"
		valor, _ := entry["V"].(string)
		indicator, _ := entry["D3N"].(string)

		records = append(records, domain.SourceRecord{
			Source:    source,
			RecordKey: period,
			Data: map[string]any{
				"periodo":       period,
				"periodo_nome":  periodName,
				"variacao_pct":  valor,
				"indicador":     indicator,
			},
			RawData:   entry,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
