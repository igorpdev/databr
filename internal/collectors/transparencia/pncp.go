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

const pncpBase = "https://pncp.gov.br/api/pncp/v1"

// PNCPCollector fetches public procurement data from PNCP (Portal Nacional de Contratações Públicas).
// Note: /contratos endpoint has a routing bug on the server — use /contratacoes instead.
type PNCPCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewPNCPCollector creates a PNCP collector.
func NewPNCPCollector(baseURL string) *PNCPCollector {
	if baseURL == "" {
		baseURL = pncpBase
	}
	return &PNCPCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *PNCPCollector) Source() string   { return "pncp_licitacoes" }
func (c *PNCPCollector) Schedule() string { return "@daily" }

// Collect fetches today's published contratacoes from PNCP.
func (c *PNCPCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	var url string
	if strings.Contains(c.baseURL, "pncp.gov.br") {
		url = fmt.Sprintf(
			"%s/contratacoes/publicacao?dataInicial=%s&dataFinal=%s&pagina=1&tamanhoPagina=100",
			c.baseURL,
			yesterday.Format("2006-01-02"),
			now.Format("2006-01-02"),
		)
	} else {
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pncp: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pncp: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pncp: upstream returned %d", resp.StatusCode)
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("pncp: decode: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(raw))
	for _, entry := range raw {
		numControle, _ := entry["numeroControlePNCP"].(string)
		if numControle == "" {
			continue
		}
		objeto, _ := entry["objeto"].(string)
		valor, _ := entry["valorTotalEstimado"].(float64)
		dataPubl, _ := entry["dataPublicacaoGlobal"].(string)

		records = append(records, domain.SourceRecord{
			Source:    "pncp_licitacoes",
			RecordKey: numControle,
			Data: map[string]any{
				"numero_controle": numControle,
				"objeto":          objeto,
				"valor_estimado":  valor,
				"data_publicacao": dataPubl,
			},
			RawData:   entry,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
