// Package cvm implements collectors for the CVM (Comissão de Valores Mobiliários).
package cvm

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// CVM IPE CSV column indices (0-based).
// Header: CNPJ_Companhia;Nome_Companhia;Codigo_CVM;Data_Referencia;Categoria;
//         Tipo;Especie;Assunto;Data_Entrega;Tipo_Apresentacao;Protocolo_Entrega;Versao;Link_Download
const (
	colCNPJCompanhia    = 0
	colNomeCompanhia    = 1
	colCodigoCVM        = 2
	colDataReferencia   = 3
	colCategoria        = 4
	colTipo             = 5
	colEspecie          = 6
	colAssunto          = 7
	colDataEntrega      = 8
	colTipoApresentacao = 9
	colProtocolo        = 10
	colVersao           = 11
	colLinkDownload     = 12
	numIPECols          = 13
)

// categoriaFatoRelevante is the exact Categoria value for fatos relevantes in the
// CVM IPE CSV. The comparison is exact (not substring) because other categories can
// share the words "Fato Relevante" in their description.
const categoriaFatoRelevante = "Fato Relevante"

// FatosRelevantesCollector downloads the annual CVM IPE ZIP file and extracts
// only the "Fato Relevante" filings. The file is ISO-8859-1 encoded and
// semicolon-delimited with CRLF line endings.
//
// Source:   "cvm_fatos"
// Schedule: "@monthly"
// RecordKey: Protocolo_Entrega (unique filing protocol number)
type FatosRelevantesCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewFatosRelevantesCollector creates a CVM fatos relevantes collector.
// baseURL overrides the production base URL (useful for tests); if empty,
// the production CVM open-data URL is used.
func NewFatosRelevantesCollector(baseURL string) *FatosRelevantesCollector {
	if baseURL == "" {
		baseURL = cvmBase
	}
	return &FatosRelevantesCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *FatosRelevantesCollector) Source() string   { return "cvm_fatos" }
func (c *FatosRelevantesCollector) Schedule() string { return "0 12 * * 1" }

// Collect downloads and parses the current year's IPE ZIP file, returning only
// "Fato Relevante" records. Falls back to the previous year if the current year
// file is not yet available.
func (c *FatosRelevantesCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "dados.cvm.gov.br") {
		year := time.Now().Year()
		url = fmt.Sprintf("%s/CIA_ABERTA/DOC/IPE/DADOS/ipe_cia_aberta_%d.zip", c.baseURL, year)
	} else {
		// In tests or custom servers, hit the base URL directly.
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cvm_fatos: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cvm_fatos: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cvm_fatos: upstream returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cvm_fatos: read body: %w", err)
	}

	return parseFatosZip(body)
}

// parseFatosZip extracts the CSV from the ZIP archive and filters for
// "Fato Relevante" rows.
func parseFatosZip(zipData []byte) ([]domain.SourceRecord, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("cvm_fatos: open zip: %w", err)
	}

	var records []domain.SourceRecord
	for _, f := range r.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("cvm_fatos: open %s: %w", f.Name, err)
		}
		rows, err := parseFatosCSV(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		records = append(records, rows...)
	}
	return records, nil
}

// parseFatosCSV reads a semicolon-delimited, ISO-8859-1 CSV reader and returns
// only the rows where Categoria == "Fato Relevante".
func parseFatosCSV(r io.Reader) ([]domain.SourceRecord, error) {
	// Wrap in ISO-8859-1 → UTF-8 decoder.
	decoded := transform.NewReader(r, charmap.ISO8859_1.NewDecoder())

	csvReader := csv.NewReader(decoded)
	csvReader.Comma = ';'
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1 // tolerate irregular rows

	// Read and discard the header row.
	if _, err := csvReader.Read(); err != nil {
		return nil, fmt.Errorf("cvm_fatos: read header: %w", err)
	}

	var records []domain.SourceRecord
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}
		if len(row) < numIPECols {
			continue
		}

		categoria := strings.TrimSpace(row[colCategoria])
		if categoria != categoriaFatoRelevante {
			continue
		}

		protocolo := strings.TrimSpace(row[colProtocolo])
		if protocolo == "" {
			continue
		}

		cnpj := strings.TrimSpace(row[colCNPJCompanhia])

		data := map[string]any{
			"cnpj":              cnpj,
			"empresa":           strings.TrimSpace(row[colNomeCompanhia]),
			"codigo_cvm":        strings.TrimSpace(row[colCodigoCVM]),
			"categoria":         categoria,
			"tipo":              strings.TrimSpace(row[colTipo]),
			"especie":           strings.TrimSpace(row[colEspecie]),
			"assunto":           strings.TrimSpace(row[colAssunto]),
			"data_referencia":   strings.TrimSpace(row[colDataReferencia]),
			"data_entrega":      strings.TrimSpace(row[colDataEntrega]),
			"tipo_apresentacao": strings.TrimSpace(row[colTipoApresentacao]),
			"protocolo":         protocolo,
			"versao":            strings.TrimSpace(row[colVersao]),
			"link_download":     strings.TrimSpace(row[colLinkDownload]),
		}

		records = append(records, domain.SourceRecord{
			Source:    "cvm_fatos",
			RecordKey: protocolo,
			Data:      data,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
