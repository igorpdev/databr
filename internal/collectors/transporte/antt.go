package transporte

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const (
	anttCKANPackageURL = "https://dados.antt.gov.br/api/3/action/package_show?id=rntrc"
)

// CSV column indices (0-based) for the ANTT RNTRC dataset.
// Header: nome_transportador;numero_rntrc;data_primeiro_cadastro;situacao_rntrc;
//
//	cpfcnpjtransportador;categoria_transportador;cep;municipio;uf;equiparado;data_situacao_rntrc
const (
	colNomeTransportador   = 0
	colNumeroRNTRC         = 1
	colDataPrimeiroCadastro = 2
	colSituacaoRNTRC       = 3
	colCPFCNPJ             = 4
	colCategoriaTransportador = 5
	colCEP                 = 6
	colMunicipio           = 7
	colUF                  = 8
	colEquiparado          = 9
	colDataSituacaoRNTRC   = 10
	anttNumCols            = 11
)

// ckanPackageResult is the minimal subset of the CKAN package_show response needed
// to discover the latest CSV download URL.
type ckanPackageResult struct {
	Result struct {
		Resources []struct {
			URL    string `json:"url"`
			Name   string `json:"name"`
			Format string `json:"format"`
		} `json:"resources"`
	} `json:"result"`
}

// ANTTCollector fetches RNTRC (Registro Nacional de Transportadores Rodoviários de
// Carga) data from the ANTT open-data CKAN portal. The dataset is updated monthly
// and contains hundreds of thousands of registered road cargo carriers.
//
// On each Collect call the collector:
//  1. Queries the CKAN API to find the URL of the most-recent CSV resource.
//  2. Downloads and streams the CSV, producing one SourceRecord per carrier.
//
// Source: "antt_rntrc"
// Schedule: "@monthly"
// RecordKey: numero_rntrc (9-digit string, e.g. "000000001")
type ANTTCollector struct {
	// ckanURL overrides the CKAN package_show URL (useful for tests).
	ckanURL    string
	// csvURL, if non-empty, skips CKAN discovery and uses this URL directly (useful for tests).
	csvURL     string
	httpClient *http.Client
}

// NewANTTCollector creates an ANTT RNTRC collector.
//   - ckanURL overrides the CKAN package_show URL; pass "" to use the production URL.
//   - csvURL, if non-empty, bypasses CKAN discovery and downloads from this URL directly.
//     Pass "" in production; pass a test-server URL in tests.
func NewANTTCollector(ckanURL, csvURL string) *ANTTCollector {
	if ckanURL == "" {
		ckanURL = anttCKANPackageURL
	}
	return &ANTTCollector{
		ckanURL: ckanURL,
		csvURL:  csvURL,
		httpClient: &http.Client{
			Timeout: 180 * time.Second, // large monthly CSV
		},
	}
}

func (c *ANTTCollector) Source() string   { return "antt_rntrc" }
func (c *ANTTCollector) Schedule() string { return "0 11 10 * *" }

// Collect discovers the latest RNTRC CSV via the CKAN API, then streams and parses it.
func (c *ANTTCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	csvURL := c.csvURL
	if csvURL == "" {
		var err error
		csvURL, err = c.discoverCSVURL(ctx)
		if err != nil {
			log.Printf("[WARN] antt_rntrc: CKAN discovery failed: %v — skipping collection", err)
			return nil, nil
		}
	}

	return c.fetchAndParse(ctx, csvURL)
}

// discoverCSVURL calls the CKAN package_show API and returns the URL of the most-recent
// CSV resource. The resources array is ordered oldest-first, so the last CSV entry is
// the most recent monthly file.
func (c *ANTTCollector) discoverCSVURL(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ckanURL, nil)
	if err != nil {
		return "", fmt.Errorf("build CKAN request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("CKAN request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("CKAN returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read CKAN response: %w", err)
	}

	var pkg ckanPackageResult
	if err := json.Unmarshal(body, &pkg); err != nil {
		return "", fmt.Errorf("parse CKAN JSON: %w", err)
	}

	// Iterate in reverse to find the most-recent CSV resource (last = newest).
	resources := pkg.Result.Resources
	for i := len(resources) - 1; i >= 0; i-- {
		r := resources[i]
		if strings.EqualFold(r.Format, "CSV") && r.URL != "" {
			return r.URL, nil
		}
	}

	return "", fmt.Errorf("no CSV resource found in CKAN package")
}

// fetchAndParse downloads the CSV at csvURL and converts each data row into a
// domain.SourceRecord.
//
// The CSV uses semicolons as delimiters with optionally-quoted fields (some fields
// like nome_transportador are quoted, others are not). The file is encoded in
// ISO-8859-1 / Latin-1 — we transcode to UTF-8 before storing.
func (c *ANTTCollector) fetchAndParse(ctx context.Context, csvURL string) ([]domain.SourceRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, csvURL, nil)
	if err != nil {
		return nil, fmt.Errorf("antt_rntrc: build CSV request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("antt_rntrc: fetch CSV: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("antt_rntrc: upstream returned HTTP %d", resp.StatusCode)
	}

	// Transcode from ISO-8859-1 to UTF-8.
	utf8Body := transform.NewReader(resp.Body, charmap.ISO8859_1.NewDecoder())
	return parseANTTCSV(utf8Body, c.Source())
}

// parseANTTCSV reads the ANTT RNTRC CSV line by line and produces SourceRecords.
// The CSV uses semicolons as delimiters; fields may or may not be quoted with
// double-quotes. Parsing is line-oriented (using bufio.Scanner) for memory efficiency.
func parseANTTCSV(body io.Reader, source string) ([]domain.SourceRecord, error) {
	scanner := bufio.NewScanner(body)
	// Buffer up to 512 KB per line to handle long company names.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 512*1024)

	// Skip header line.
	if !scanner.Scan() {
		return nil, nil
	}

	now := time.Now().UTC()
	var records []domain.SourceRecord

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := splitSemicolonQuotedANTT(line)
		if len(fields) < anttNumCols {
			// Skip malformed rows rather than aborting the entire import.
			continue
		}

		rntrc := strings.TrimSpace(fields[colNumeroRNTRC])
		if rntrc == "" {
			continue
		}

		cpfCNPJ := strings.TrimSpace(fields[colCPFCNPJ])
		cpfCNPJDigits := onlyDigits(cpfCNPJ)

		records = append(records, domain.SourceRecord{
			Source:    source,
			RecordKey: rntrc,
			Data: map[string]any{
				"nome":            strings.TrimSpace(fields[colNomeTransportador]),
				"rntrc":           rntrc,
				"situacao":        strings.TrimSpace(fields[colSituacaoRNTRC]),
				"categoria":       strings.TrimSpace(fields[colCategoriaTransportador]),
				"cpf_cnpj":        cpfCNPJ,
				"cpf_cnpj_digits": cpfCNPJDigits,
				"cep":             strings.TrimSpace(fields[colCEP]),
				"municipio":       strings.TrimSpace(fields[colMunicipio]),
				"uf":              strings.TrimSpace(fields[colUF]),
				"equiparado":      strings.TrimSpace(fields[colEquiparado]),
				"data_cadastro":   strings.TrimSpace(fields[colDataPrimeiroCadastro]),
				"data_situacao":   strings.TrimSpace(fields[colDataSituacaoRNTRC]),
			},
			FetchedAt: now,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning ANTT CSV: %w", err)
	}

	return records, nil
}

// splitSemicolonQuotedANTT splits a semicolon-delimited line, stripping optional
// surrounding double-quotes from each field. Unlike the ANEEL file (which quotes ALL
// fields), the ANTT RNTRC CSV quotes only some fields (e.g. nome_transportador) and
// leaves numeric/code fields unquoted. This function handles both cases.
func splitSemicolonQuotedANTT(line string) []string {
	parts := strings.Split(line, ";")
	fields := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
			p = p[1 : len(p)-1]
		}
		// Unescape doubled double-quotes ("" → ").
		p = strings.ReplaceAll(p, `""`, `"`)
		fields = append(fields, p)
	}
	return fields
}

// onlyDigits returns a string containing only the digit characters of s.
// Used to normalise CPF/CNPJ values like "11.193.322/0001-10" → "11193322000110"
// so they can be searched via JSONB equality.
func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
