// Package bcb implements collectors for the Banco Central do Brasil APIs.
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

const (
	// focusOLINDABase is the BCB OLINDA base URL for the Expectativas service.
	// Service name is CASE-SENSITIVE: "Expectativas" (not "expectativas").
	focusOLINDABase = "https://olinda.bcb.gov.br/olinda/servico/Expectativas/versao/v1/odata"

	// focusEntitySet is the correct EntitySet name from the OLINDA $metadata.
	// NOTE: "ExpectativasMercadoAnuais" — NOT "ExpectativaMercadoAnual" (that is the EntityType).
	focusEntitySet = "ExpectativasMercadoAnuais"

	// focusTopN limits how many records we fetch per call.
	// Annual expectations have many historical rows; we request the most recent.
	focusTopN = 100
)

// FocusCollector fetches Relatório Focus (market expectations) from BCB OLINDA.
// It collects annual market expectations for key indicators: IPCA, PIB, Câmbio, etc.
type FocusCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewFocusCollector creates a Focus collector.
// baseURL may be overridden for testing; if empty the production BCB OLINDA URL is used.
func NewFocusCollector(baseURL string) *FocusCollector {
	if baseURL == "" {
		baseURL = focusOLINDABase
	}
	return &FocusCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *FocusCollector) Source() string   { return "bcb_focus" }
func (c *FocusCollector) Schedule() string { return "0 12 * * 1" }

// Collect fetches the latest annual market expectations from the BCB Relatório Focus.
// Records are keyed by "INDICADOR_DATAREFERENCIA" (e.g., "IPCA_2026").
// Only the most recent survey date is returned per indicator+year combination.
func (c *FocusCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "olinda.bcb.gov.br") {
		// Production: filter for the last 30 days of survey dates, most recent first.
		since := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		url = fmt.Sprintf(
			"%s/%s?$top=%d&$format=json&$filter=Data%%20ge%%20'%s'&$orderby=Data%%20desc",
			c.baseURL, focusEntitySet, focusTopN, since,
		)
	} else {
		// Test server: use plain URL (test server handles all paths).
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bcb_focus: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bcb_focus: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bcb_focus: upstream returned %d", resp.StatusCode)
	}

	var raw struct {
		Value []struct {
			Indicador          string  `json:"Indicador"`
			IndicadorDetalhe   string  `json:"IndicadorDetalhe"`
			Data               string  `json:"Data"`
			DataReferencia     string  `json:"DataReferencia"`
			Media              float64 `json:"Media"`
			Mediana            float64 `json:"Mediana"`
			DesvioPadrao       float64 `json:"DesvioPadrao"`
			Minimo             float64 `json:"Minimo"`
			Maximo             float64 `json:"Maximo"`
			NumeroRespondentes int     `json:"numeroRespondentes"`
			BaseCalculo        int     `json:"baseCalculo"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bcb_focus: decode: %w", err)
	}

	if len(raw.Value) == 0 {
		return []domain.SourceRecord{}, nil
	}

	// De-duplicate: keep only the most recent survey date per (Indicador, IndicadorDetalhe, DataReferencia).
	// Since results are ordered by Data desc, the first occurrence wins.
	type key struct {
		indicador        string
		indicadorDetalhe string
		dataReferencia   string
	}
	seen := make(map[key]bool)
	records := make([]domain.SourceRecord, 0, len(raw.Value))

	for _, entry := range raw.Value {
		k := key{
			indicador:        entry.Indicador,
			indicadorDetalhe: entry.IndicadorDetalhe,
			dataReferencia:   entry.DataReferencia,
		}
		if seen[k] {
			continue
		}
		seen[k] = true

		// Build a stable RecordKey: "INDICADOR_DATAREFERENCIA"
		// When there is a detalhe (e.g. "Exportações"), include it for uniqueness.
		recordKey := buildFocusRecordKey(entry.Indicador, entry.IndicadorDetalhe, entry.DataReferencia)

		data := map[string]any{
			"indicador":          entry.Indicador,
			"data_referencia":    entry.DataReferencia,
			"data":               entry.Data,
			"media":              entry.Media,
			"mediana":            entry.Mediana,
			"desvio_padrao":      entry.DesvioPadrao,
			"minimo":             entry.Minimo,
			"maximo":             entry.Maximo,
			"numero_respondentes": entry.NumeroRespondentes,
			"base_calculo":       entry.BaseCalculo,
		}
		if entry.IndicadorDetalhe != "" {
			data["indicador_detalhe"] = entry.IndicadorDetalhe
		}

		records = append(records, domain.SourceRecord{
			Source:    "bcb_focus",
			RecordKey: recordKey,
			Data:      data,
			FetchedAt: time.Now().UTC(),
		})
	}

	return records, nil
}

// buildFocusRecordKey builds a stable, unique record key for a Focus expectation entry.
// Format: "INDICADOR_DATAREFERENCIA" or "INDICADOR_DETALHE_DATAREFERENCIA" when a detail exists.
// Spaces are replaced with underscores; accented characters are preserved.
func buildFocusRecordKey(indicador, indicadorDetalhe, dataReferencia string) string {
	ind := strings.ReplaceAll(indicador, " ", "_")
	if indicadorDetalhe != "" {
		det := strings.ReplaceAll(indicadorDetalhe, " ", "_")
		return fmt.Sprintf("%s_%s_%s", ind, det, dataReferencia)
	}
	return fmt.Sprintf("%s_%s", ind, dataReferencia)
}
