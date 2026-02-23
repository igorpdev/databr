package ibge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const (
	pibTabela = "5932" // PIB trimestral a preços de mercado
	pibVar    = "6561" // variável principal
)

// PIBCollector fetches GDP data from IBGE SIDRA table 5932.
type PIBCollector struct {
	baseURL    string
	ipca       *IPCACollector // reuses HTTP client and fetch logic
}

// NewPIBCollector creates a PIB collector.
func NewPIBCollector(baseURL string) *PIBCollector {
	if baseURL == "" {
		baseURL = sidraBase
	}
	return &PIBCollector{
		baseURL: strings.TrimRight(baseURL, "/"),
		ipca:    NewIPCACollector(baseURL),
	}
}

func (c *PIBCollector) Source() string   { return "ibge_pib" }
func (c *PIBCollector) Schedule() string { return "0 14 * * 1-5" }

// Collect fetches the last 4 quarters of PIB data.
func (c *PIBCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "servicodados.ibge.gov.br") {
		localidades := "%5Ball%5D"
		url = fmt.Sprintf("%s/%s/periodos/-4/variaveis/%s?localidades=N1%s&view=flat",
			c.baseURL, pibTabela, pibVar, localidades)
	} else {
		url = c.baseURL
	}

	raw, err := c.ipca.fetch(ctx, url, "ibge_pib")
	if err != nil {
		return nil, err
	}

	// Rename variacao_pct → valor for PIB (it's an absolute value, not a percentage)
	for i := range raw {
		if v, ok := raw[i].Data["variacao_pct"]; ok {
			raw[i].Data["valor"] = v
			delete(raw[i].Data, "variacao_pct")
		}
	}
	return raw, nil
}

// lastBusinessDay returns the most recent business day (Mon-Fri) before or on date.
func lastBusinessDay(t time.Time) time.Time {
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		t = t.AddDate(0, 0, -1)
	}
	return t
}
