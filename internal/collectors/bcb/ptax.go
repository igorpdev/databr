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

const olindaBase = "https://olinda.bcb.gov.br/olinda/servico"

// PTAXCollector fetches exchange rates from BCB OLINDA (PTAX service).
// Service name is case-sensitive: "PTAX" (not "ptax").
type PTAXCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewPTAXCollector creates a PTAX collector.
func NewPTAXCollector(baseURL string) *PTAXCollector {
	if baseURL == "" {
		baseURL = olindaBase
	}
	return &PTAXCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *PTAXCollector) Source() string   { return "bcb_ptax" }
func (c *PTAXCollector) Schedule() string { return "@daily" }

// Collect fetches USD PTAX for the most recent available trading day.
// PTAX returns empty on weekends and holidays, so we walk back up to 7 days.
func (c *PTAXCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		records, err := c.FetchByCurrency(ctx, "USD", date)
		if err != nil {
			return nil, err
		}
		if len(records) > 0 {
			return records, nil
		}
	}
	// All 7 days were holidays/weekends — return empty (not an error)
	return []domain.SourceRecord{}, nil
}

// FetchByCurrency fetches the PTAX rate for the given currency (e.g. "USD", "EUR")
// on the given date (YYYY-MM-DD format). Returns empty slice for weekends/holidays.
func (c *PTAXCollector) FetchByCurrency(ctx context.Context, currency, date string) ([]domain.SourceRecord, error) {
	// OLINDA date format: MM-DD-YYYY
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("bcb_ptax: invalid date %q: %w", date, err)
	}
	olindaDate := t.Format("01-02-2006")

	var url string
	if strings.Contains(c.baseURL, "olinda.bcb.gov.br") {
		if currency == "USD" {
			url = fmt.Sprintf(
				"%s/PTAX/versao/v1/odata/CotacaoDolarDia(dataCotacao=@dataCotacao)?@dataCotacao='%s'&$format=json",
				c.baseURL, olindaDate,
			)
		} else {
			url = fmt.Sprintf(
				"%s/PTAX/versao/v1/odata/CotacaoMoedaDia(moeda=@moeda,dataCotacao=@dataCotacao)?@moeda='%s'&@dataCotacao='%s'&$format=json",
				c.baseURL, currency, olindaDate,
			)
		}
	} else {
		// Test server: plain URL
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bcb_ptax: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bcb_ptax: fetch %s: %w", currency, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bcb_ptax: upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Value []map[string]any `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bcb_ptax: decode: %w", err)
	}

	// Empty value array = weekend or holiday — this is expected, not an error
	if len(raw.Value) == 0 {
		return []domain.SourceRecord{}, nil
	}

	entry := raw.Value[0]
	compra, _ := entry["cotacaoCompra"].(float64)
	venda, _ := entry["cotacaoVenda"].(float64)
	dataHora, _ := entry["dataHoraCotacao"].(string)

	return []domain.SourceRecord{
		{
			Source:    "bcb_ptax",
			RecordKey: fmt.Sprintf("%s_%s", currency, date),
			Data: map[string]any{
				"moeda":          currency,
				"data":           date,
				"cotacao_compra": compra,
				"cotacao_venda":  venda,
				"data_hora":      dataHora,
			},
			FetchedAt: time.Now().UTC(),
		},
	}, nil
}
