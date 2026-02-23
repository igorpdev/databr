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

const taxaJurosBase = "https://olinda.bcb.gov.br/olinda/servico/TaxaJuros/versao/v2/odata"

// TaxasCreditoCollector fetches credit interest rates from BCB OLINDA (TaxaJuros service v2).
type TaxasCreditoCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewTaxasCreditoCollector creates a credit rates collector.
// Pass empty string for baseURL to use the production OLINDA endpoint.
func NewTaxasCreditoCollector(baseURL string) *TaxasCreditoCollector {
	if baseURL == "" {
		baseURL = taxaJurosBase
	}
	return &TaxasCreditoCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *TaxasCreditoCollector) Source() string   { return "bcb_taxas_credito" }
func (c *TaxasCreditoCollector) Schedule() string { return "@daily" }

// Collect fetches the latest 50 credit market interest rate records from BCB OLINDA.
func (c *TaxasCreditoCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "olinda.bcb.gov.br") {
		url = fmt.Sprintf(
			"%s/TaxasJurosMercadoCredito?$format=json&$top=50&$orderby=DataReferencia%%20desc",
			c.baseURL,
		)
	} else {
		// Test server: use base URL directly
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bcb_taxas_credito: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bcb_taxas_credito: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bcb_taxas_credito: upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Value []map[string]any `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bcb_taxas_credito: decode: %w", err)
	}

	if len(raw.Value) == 0 {
		return nil, fmt.Errorf("bcb_taxas_credito: no records returned")
	}

	records := make([]domain.SourceRecord, 0, len(raw.Value))
	for _, entry := range raw.Value {
		segmento, _ := entry["Segmento"].(string)
		modalidade, _ := entry["Modalidade"].(string)
		posicao, _ := entry["Posicao"].(string)
		dataRef, _ := entry["DataReferencia"].(string)
		taxaMensal, _ := entry["TaxaJurosMensal"].(float64)
		taxaAnual, _ := entry["TaxaJurosAnual"].(float64)

		if modalidade == "" || dataRef == "" {
			continue
		}

		// RecordKey: "{Modalidade}_{DataReferencia}" — uniquely identifies a rate entry
		key := fmt.Sprintf("%s_%s", modalidade, dataRef)

		records = append(records, domain.SourceRecord{
			Source:    "bcb_taxas_credito",
			RecordKey: key,
			Data: map[string]any{
				"segmento":       segmento,
				"modalidade":     modalidade,
				"posicao":        posicao,
				"data_referencia": dataRef,
				"taxa_mensal":    taxaMensal,
				"taxa_anual":     taxaAnual,
			},
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
