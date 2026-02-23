package tesouro

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

// tesouroDiretoBase is the Tesouro Transparente CKAN direct CSV download.
// Contains the full historical series of Tesouro Direto prices and rates.
const tesouroDiretoBase = "https://www.tesourotransparente.gov.br/ckan/dataset/df56aa42-484a-4a59-8184-7676580c81e3/resource/796d2059-14e9-44e3-80c9-2d9e30b405c1/download/precotaxatesourodireto.csv"

// TesouroDiretoCollector fetches Tesouro Direto bond prices and rates.
type TesouroDiretoCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewTesouroDiretoCollector creates a Tesouro Direto collector.
// Pass empty string for baseURL to use the Tesouro Transparente production endpoint.
func NewTesouroDiretoCollector(baseURL string) *TesouroDiretoCollector {
	if baseURL == "" {
		baseURL = tesouroDiretoBase
	}
	return &TesouroDiretoCollector{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *TesouroDiretoCollector) Source() string   { return "tesouro_titulos" }
func (c *TesouroDiretoCollector) Schedule() string { return "0 21 * * 1-5" }

// Collect fetches the latest Tesouro Direto prices from Tesouro Transparente.
// The CSV holds the full history; only the most recent Data Base date is returned.
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

	records, err := parseTitulosCSV(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tesouro_titulos: %w", err)
	}
	return records, nil
}

// parseTitulosCSV parses the Tesouro Transparente semicolon-delimited CSV and
// returns only records for the most recent Data Base date seen in the file.
// Columns: Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha
func parseTitulosCSV(r io.Reader) ([]domain.SourceRecord, error) {
	csvReader := csv.NewReader(r)
	csvReader.Comma = ';'
	csvReader.LazyQuotes = true

	if _, err := csvReader.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var (
		latestISO string                  // yyyy-mm-dd of the latest Data Base seen
		current   []domain.SourceRecord   // records for latestISO
	)

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) < 8 {
			continue
		}

		tipo := strings.TrimSpace(row[0])
		dataVenc := strings.TrimSpace(row[1])
		dataBase := strings.TrimSpace(row[2])
		if tipo == "" || dataBase == "" {
			continue
		}

		dateISO := brDateToISO(dataBase)

		if dateISO > latestISO {
			latestISO = dateISO
			current = current[:0] // discard older records
		}

		if dateISO == latestISO {
			key := strings.ReplaceAll(tipo, " ", "_") + "_" + strings.ReplaceAll(dataVenc, "/", "-")
			current = append(current, domain.SourceRecord{
				Source:    "tesouro_titulos",
				RecordKey: key,
				Data: map[string]any{
					"nome":        tipo,
					"vencimento":  dataVenc,
					"data_base":   dataBase,
					"taxa_compra": parseBRFloat(row[3]),
					"taxa_venda":  parseBRFloat(row[4]),
					"pu_compra":   parseBRFloat(row[5]),
					"pu_venda":    parseBRFloat(row[6]),
					"pu_base":     parseBRFloat(row[7]),
				},
				FetchedAt: time.Now().UTC(),
			})
		}
	}

	if len(current) == 0 {
		return nil, fmt.Errorf("no records in CSV")
	}
	return current, nil
}

// brDateToISO converts a Brazilian dd/mm/yyyy date string to yyyy-mm-dd for comparison.
func brDateToISO(s string) string {
	parts := strings.Split(strings.TrimSpace(s), "/")
	if len(parts) != 3 {
		return s
	}
	return parts[2] + "-" + parts[1] + "-" + parts[0]
}

// parseBRFloat parses a Brazilian-formatted float (comma as decimal separator).
func parseBRFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	return v
}
