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

const tesouroDiretoBase = "https://www.tesourodireto.com.br/json/br/com/b3/tesourodireto/model/json/TreasureBondsFile.json"

// TesouroDiretoCollector fetches public Tesouro Direto bond prices and rates.
type TesouroDiretoCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewTesouroDiretoCollector creates a Tesouro Direto collector.
// Pass empty string for baseURL to use the production endpoint.
func NewTesouroDiretoCollector(baseURL string) *TesouroDiretoCollector {
	if baseURL == "" {
		baseURL = tesouroDiretoBase
	}
	return &TesouroDiretoCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *TesouroDiretoCollector) Source() string   { return "tesouro_titulos" }
func (c *TesouroDiretoCollector) Schedule() string { return "@daily" }

// Collect fetches current Tesouro Direto bond prices from the public JSON endpoint.
func (c *TesouroDiretoCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("tesouro_titulos: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tesouro_titulos: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tesouro_titulos: upstream returned %d", resp.StatusCode)
	}

	// The API response wraps everything under "response.TrsrBdTradgList"
	var raw struct {
		Response struct {
			TrsrBdTradgList []struct {
				TrsrBd map[string]any `json:"TrsrBd"`
			} `json:"TrsrBdTradgList"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("tesouro_titulos: decode: %w", err)
	}

	bonds := raw.Response.TrsrBdTradgList
	if len(bonds) == 0 {
		return nil, fmt.Errorf("tesouro_titulos: no bonds returned")
	}

	records := make([]domain.SourceRecord, 0, len(bonds))
	for _, item := range bonds {
		bd := item.TrsrBd
		if bd == nil {
			continue
		}

		nm, _ := bd["nm"].(string)
		if nm == "" {
			continue
		}

		mtrtyDt, _ := bd["mtrtyDt"].(string)
		untrRedVal, _ := bd["untrRedVal"].(float64)
		anulInvstmtRate, _ := bd["anulInvstmtRate"].(float64)
		anulRedRate, _ := bd["anulRedRate"].(float64)
		minInvstmtAmt, _ := bd["minInvstmtAmt"].(float64)

		// RecordKey: bond name with spaces replaced by underscores
		key := strings.ReplaceAll(nm, " ", "_")

		records = append(records, domain.SourceRecord{
			Source:    "tesouro_titulos",
			RecordKey: key,
			Data: map[string]any{
				"nome":                    nm,
				"vencimento":              mtrtyDt,
				"taxa_anual_compra":       anulInvstmtRate,
				"taxa_anual_resgate":      anulRedRate,
				"preco_minimo":            minInvstmtAmt,
				"preco_unitario_resgate":  untrRedVal,
			},
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
