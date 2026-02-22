// Package tesouro implements collectors for Tesouro Nacional (SICONFI).
package tesouro

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

// siconfiBase is the production SICONFI API base URL.
// The path ending in "tt/" is correct — the double slash "tt//" in FetchRREO
// is intentional ORDS routing and must not be removed.
const siconfiBase = "https://apidatalake.tesouro.gov.br/ords/siconfi/tt/"

// SICONFICollector fetches fiscal data from SICONFI (Tesouro Nacional).
type SICONFICollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewSICONFICollector creates a SICONFI collector.
// Pass empty string for baseURL to use the production SICONFI endpoint.
func NewSICONFICollector(baseURL string) *SICONFICollector {
	if baseURL == "" {
		baseURL = siconfiBase
	}
	return &SICONFICollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *SICONFICollector) Source() string   { return "tesouro_siconfi" }
func (c *SICONFICollector) Schedule() string { return "@monthly" }

// Collect is a no-op — SICONFI is fetched on-demand per UF/ano/período.
func (c *SICONFICollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	return nil, nil
}

// ufToIBGE maps Brazilian state abbreviations to their 2-digit IBGE codes.
// SICONFI requires id_ente (IBGE code), not the UF abbreviation.
var ufToIBGE = map[string]int{
	"AC": 12, "AL": 27, "AM": 13, "AP": 16, "BA": 29, "CE": 23,
	"DF": 53, "ES": 32, "GO": 52, "MA": 21, "MG": 31, "MS": 50,
	"MT": 51, "PA": 15, "PB": 25, "PE": 26, "PI": 22, "PR": 41,
	"RJ": 33, "RN": 24, "RO": 11, "RR": 14, "RS": 43, "SC": 42,
	"SE": 28, "SP": 35, "TO": 17,
}

// FetchRREO fetches the RREO (Relatório Resumido da Execução Orçamentária)
// for a given UF abbreviation (ex: SP), exercise year and period number.
// Note: when using the production URL, the double slash (tt//) is intentional ORDS routing.
func (c *SICONFICollector) FetchRREO(ctx context.Context, uf string, ano, periodo int) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "apidatalake.tesouro.gov.br") {
		// SICONFI uses id_ente (IBGE 2-digit code), not UF abbreviation
		idEnte, ok := ufToIBGE[strings.ToUpper(uf)]
		if !ok {
			return nil, fmt.Errorf("siconfi: UF desconhecida: %s", uf)
		}
		// Double slash after "tt/" is intentional — required by ORDS routing
		url = fmt.Sprintf(
			"%s//rreo?an_exercicio=%d&nr_periodo=%d&co_tipo_demonstrativo=RREO&id_ente=%d",
			c.baseURL, ano, periodo, idEnte,
		)
	} else {
		// Test server: use baseURL directly
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("siconfi: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("siconfi: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("siconfi: upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("siconfi: decode: %w", err)
	}

	if len(raw.Items) == 0 {
		return nil, nil
	}
	// Return all rows as a single aggregated record (keyed by UF+ano+periodo)
	key := fmt.Sprintf("%s_%d_%d", strings.ToUpper(uf), ano, periodo)
	record := domain.SourceRecord{
		Source:    "tesouro_siconfi",
		RecordKey: key,
		Data: map[string]any{
			"uf":      strings.ToUpper(uf),
			"ano":     ano,
			"periodo": periodo,
			"linhas":  raw.Items,
			"total":   len(raw.Items),
		},
		FetchedAt: time.Now().UTC(),
	}
	return []domain.SourceRecord{record}, nil
}
