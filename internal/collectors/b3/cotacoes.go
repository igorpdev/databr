// Package b3 implements collectors for B3 (Brasil, Bolsa, Balcão) market data.
// Cotações are served as BDIN fixed-width text files inside ZIP archives.
package b3

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const b3Base = "https://bvmf.bmfbovespa.com.br/InstDados/SerHist"

// CotacoesCollector downloads daily B3 stock price files (COTAHIST_D format).
type CotacoesCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewCotacoesCollector creates a B3 cotações collector.
func NewCotacoesCollector(baseURL string) *CotacoesCollector {
	if baseURL == "" {
		baseURL = b3Base
	}
	return &CotacoesCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CotacoesCollector) Source() string   { return "b3_cotacoes" }
func (c *CotacoesCollector) Schedule() string { return "@daily" }

// Collect downloads and parses B3 cotações for the last business day.
func (c *CotacoesCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "bvmf.bmfbovespa.com.br") {
		// B3 date format: DDMMYYYY (not ISO 8601)
		lastDay := LastBusinessDay(time.Now())
		filename := fmt.Sprintf("COTAHIST_D%s.ZIP", lastDay.Format("02012006"))
		url = fmt.Sprintf("%s/%s", c.baseURL, filename)
	} else {
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("b3_cotacoes: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("b3_cotacoes: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Weekend or holiday — try previous day
		return nil, fmt.Errorf("b3_cotacoes: file not found (weekend or holiday)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("b3_cotacoes: upstream returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("b3_cotacoes: read: %w", err)
	}

	return parseBDIN(body)
}

// parseBDIN parses a BDIN fixed-width ZIP file.
// Record type "01" = header, "02" = stock data, "99" = trailer.
func parseBDIN(zipData []byte) ([]domain.SourceRecord, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("b3_cotacoes: open zip: %w", err)
	}

	var records []domain.SourceRecord
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimRight(line, "\r")
			if len(line) < 13 {
				continue
			}
			recType := line[0:2]
			if recType != "02" {
				continue
			}

			// BDIN field positions (1-indexed from spec, 0-indexed here):
			// DataPregao: 2-9 (YYYYMMDD)
			// CodNeg: 12-24 (ticker, space-padded)
			// PreFech: 108-121 (close price, 13 digits, 2 decimal places)
			if len(line) < 122 {
				continue
			}

			dataPregao := strings.TrimSpace(line[2:10])
			ticker := strings.TrimSpace(line[12:24])
			preFechStr := strings.TrimSpace(line[108:121])
			abertura := strings.TrimSpace(line[56:69])
			maximo := strings.TrimSpace(line[69:82])
			minimo := strings.TrimSpace(line[82:95])

			if ticker == "" || dataPregao == "" {
				continue
			}

			// Parse date YYYYMMDD → YYYY-MM-DD
			var dataFormatada string
			if len(dataPregao) == 8 {
				dataFormatada = dataPregao[0:4] + "-" + dataPregao[4:6] + "-" + dataPregao[6:8]
			}

			records = append(records, domain.SourceRecord{
				Source:    "b3_cotacoes",
				RecordKey: fmt.Sprintf("%s_%s", ticker, dataPregao),
				Data: map[string]any{
					"ticker":      ticker,
					"data_pregao": dataFormatada,
					"preco_fech":  preFechStr,
					"preco_abert": abertura,
					"preco_max":   maximo,
					"preco_min":   minimo,
				},
				FetchedAt: time.Now().UTC(),
			})
		}
	}
	return records, nil
}

// LastBusinessDay returns the most recent trading day (Mon-Fri) on or before t.
func LastBusinessDay(t time.Time) time.Time {
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		t = t.AddDate(0, 0, -1)
	}
	return t
}
