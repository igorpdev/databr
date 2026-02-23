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

const taxaJurosBase = "https://olinda.bcb.gov.br/olinda/servico/taxaJuros/versao/v2/odata"

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
		// TaxasJurosMensalPorMes: monthly average rates by modality and financial institution
		url = fmt.Sprintf("%s/TaxasJurosMensalPorMes?$format=json&$top=200&$orderby=anoMes%%20desc", c.baseURL)
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

	now := time.Now().UTC()
	records := make([]domain.SourceRecord, 0, len(raw.Value))
	for _, entry := range raw.Value {
		mes, _ := entry["Mes"].(string)
		modalidade, _ := entry["Modalidade"].(string)
		instituicao, _ := entry["InstituicaoFinanceira"].(string)
		cnpj8, _ := entry["cnpj8"].(string)
		anoMes, _ := entry["anoMes"].(string)
		taxaMensal, _ := entry["TaxaJurosAoMes"].(float64)
		taxaAnual, _ := entry["TaxaJurosAoAno"].(float64)

		if modalidade == "" || anoMes == "" {
			continue
		}

		// RecordKey: "{cnpj8}_{anoMes}_{modalidade}" — uniquely identifies a rate entry per institution
		key := fmt.Sprintf("%s_%s_%s", cnpj8, anoMes, modalidade)

		records = append(records, domain.SourceRecord{
			Source:    "bcb_taxas_credito",
			RecordKey: key,
			Data: map[string]any{
				"mes":          mes,
				"ano_mes":      anoMes,
				"modalidade":   modalidade,
				"instituicao":  instituicao,
				"cnpj8":        cnpj8,
				"taxa_mensal":  taxaMensal,
				"taxa_anual":   taxaAnual,
			},
			FetchedAt: now,
		})
	}
	return records, nil
}
