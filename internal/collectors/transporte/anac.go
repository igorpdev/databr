// Package transporte implements collectors for Brazilian transportation data sources.
package transporte

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const (
	anacDefaultURL = "https://sistemas.anac.gov.br/dadosabertos/Aeronaves/RAB/dados_aeronaves.csv"
)

// anacColumnMap maps the CSV header names to the data keys stored in SourceRecord.Data.
// The CSV header uses the exact column names from ANAC's RAB export.
var anacColumnMap = map[string]string{
	"MARCA":              "marca",
	"PROPRIETARIOS":      "proprietarios",
	"NM_OPERADOR":        "operador",
	"OUTROS_OPERADORES":  "outros_operadores",
	"CPF_CNPJ":           "cpf_cnpj",
	"SG_UF":              "uf",
	"UF_OPERADOR":        "uf_operador",
	"NR_CERT_MATRICULA":  "cert_matricula",
	"NR_SERIE":           "nr_serie",
	"CD_TIPO":            "cd_tipo",
	"DS_MODELO":          "modelo",
	"NM_FABRICANTE":      "fabricante",
	"NR_ANO_FABRICACAO":  "ano_fabricacao",
	"DT_VALIDADE_CVA":    "validade_cva",
	"DT_VALIDADE_CA":     "validade_ca",
	"DT_MATRICULA":       "data_matricula",
	"TP_OPERACAO":        "tp_operacao",
}

// ANACCollector fetches the ANAC RAB (Registro Aeronáutico Brasileiro) open data CSV.
type ANACCollector struct {
	csvURL     string
	httpClient *http.Client
}

// NewANACCollector creates a new ANACCollector.
// csvURL overrides the production ANAC CSV URL; pass "" to use the default.
func NewANACCollector(csvURL string) *ANACCollector {
	if csvURL == "" {
		csvURL = anacDefaultURL
	}
	return &ANACCollector{
		csvURL: csvURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *ANACCollector) Source() string   { return "anac_rab" }
func (c *ANACCollector) Schedule() string { return "@weekly" }

// Collect downloads the ANAC RAB CSV and returns one SourceRecord per aircraft.
// The file is UTF-8 with BOM and uses semicolons as field separators.
func (c *ANACCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.csvURL, nil)
	if err != nil {
		return nil, fmt.Errorf("anac_rab: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anac_rab: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anac_rab: upstream returned %d", resp.StatusCode)
	}

	// Strip UTF-8 BOM (EF BB BF) if present before handing to the CSV reader.
	// We read the first 3 bytes and peek; if they match the BOM we discard them,
	// otherwise we prepend them back via io.MultiReader.
	peekBuf := make([]byte, 3)
	n, err := io.ReadFull(resp.Body, peekBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("anac_rab: peek BOM: %w", err)
	}
	peekBuf = peekBuf[:n]

	var bodyReader io.Reader
	if len(peekBuf) == 3 && peekBuf[0] == 0xEF && peekBuf[1] == 0xBB && peekBuf[2] == 0xBF {
		// BOM detected — discard those 3 bytes; the rest of resp.Body is clean UTF-8.
		bodyReader = resp.Body
	} else {
		// No BOM — put the peeked bytes back.
		bodyReader = io.MultiReader(strings.NewReader(string(peekBuf)), resp.Body)
	}

	r := csv.NewReader(bodyReader)
	r.Comma = ';'
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	// The real ANAC CSV has a metadata comment line before the actual header:
	//   Atualizado em: 2026-02-17\r\n
	// We skip rows until we find the one containing "MARCA".
	r.FieldsPerRecord = -1 // allow variable field counts while searching

	var header []string
	for {
		row, err := r.Read()
		if err != nil {
			return nil, fmt.Errorf("anac_rab: could not find header row: %w", err)
		}
		// The real header row contains "MARCA" as first field (possibly quoted).
		if len(row) > 0 && strings.TrimSpace(strings.Trim(row[0], `"`)) == "MARCA" {
			header = row
			break
		}
	}
	// Now enforce fixed field count for the rest of the file.
	r.FieldsPerRecord = len(header)

	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		// Trim any residual whitespace and surrounding quotes from header names.
		colIdx[strings.TrimSpace(strings.Trim(h, `"`))] = i
	}

	marcaIdx, hasMarca := colIdx["MARCA"]
	if !hasMarca {
		return nil, fmt.Errorf("anac_rab: MARCA column not found in header")
	}

	now := time.Now().UTC()
	var records []domain.SourceRecord

	for {
		row, err := r.Read()
		if err != nil {
			if isEOF(err) {
				break
			}
			// Skip malformed rows.
			continue
		}

		if marcaIdx >= len(row) {
			continue
		}
		marca := strings.TrimSpace(row[marcaIdx])
		if marca == "" {
			continue
		}

		data := make(map[string]any, len(anacColumnMap))
		for csvCol, dataKey := range anacColumnMap {
			if idx, ok := colIdx[csvCol]; ok && idx < len(row) {
				data[dataKey] = strings.TrimSpace(row[idx])
			}
		}

		records = append(records, domain.SourceRecord{
			Source:    "anac_rab",
			RecordKey: marca,
			Data:      data,
			FetchedAt: now,
		})
	}

	return records, nil
}

// isEOF reports whether err signals end of file.
func isEOF(err error) bool {
	return err != nil && (err.Error() == "EOF" || strings.HasSuffix(err.Error(), "EOF"))
}
