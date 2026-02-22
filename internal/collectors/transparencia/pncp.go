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

// API migrated: old /api/pncp/v1 → /api/consulta/v1
// New requirements: date format yyyyMMdd, tamanhoPagina >= 10, codigoModalidadeContratacao required.
const pncpBase = "https://pncp.gov.br/api/consulta/v1"

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

// pncpResponse wraps the paginated response from /api/consulta/v1.
type pncpResponse struct {
	Data            []map[string]any `json:"data"`
	TotalRegistros  int              `json:"totalRegistros"`
}

// Collect fetches recent published contratacoes from PNCP (Pregão Eletrônico, modalidade 6).
// API requirements (as of 2026-02): dates in yyyyMMdd, tamanhoPagina >= 10, codigoModalidadeContratacao required.
func (c *PNCPCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	var url string
	if strings.Contains(c.baseURL, "pncp.gov.br") {
		url = fmt.Sprintf(
			"%s/contratacoes/publicacao?dataInicial=%s&dataFinal=%s&pagina=1&tamanhoPagina=50&codigoModalidadeContratacao=6",
			c.baseURL,
			yesterday.Format("20060102"),
			now.Format("20060102"),
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

	var raw pncpResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("pncp: decode: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(raw.Data))
	for _, entry := range raw.Data {
		// numeroCompra + anoCompra + sequencialCompra form a unique key
		numCompra, _ := entry["numeroCompra"].(string)
		if numCompra == "" {
			continue
		}
		orgao := ""
		if o, ok := entry["orgaoEntidade"].(map[string]any); ok {
			orgao, _ = o["razaoSocial"].(string)
		}
		objeto, _ := entry["objetoCompra"].(string)
		dataAtu, _ := entry["dataAtualizacao"].(string)

		// Build unique key from orgao CNPJ + year + sequential
		cnpjOrgao := ""
		if o, ok := entry["orgaoEntidade"].(map[string]any); ok {
			cnpjOrgao, _ = o["cnpj"].(string)
		}
		ano, _ := entry["anoCompra"].(float64)
		seq, _ := entry["sequencialCompra"].(float64)
		recordKey := fmt.Sprintf("%s_%d_%d", cnpjOrgao, int(ano), int(seq))

		records = append(records, domain.SourceRecord{
			Source:    "pncp_licitacoes",
			RecordKey: recordKey,
			Data: map[string]any{
				"numero_compra":   numCompra,
				"orgao":           orgao,
				"objeto":          objeto,
				"data_atualizacao": dataAtu,
			},
			RawData:   entry,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
