// Package energia implements collectors for the Brazilian energy sector (ANEEL).
package energia

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const (
	// aneelTarifasURL is the direct download URL for the ANEEL homologated distributor
	// tariffs CSV file. Dataset: "tarifas-distribuidoras-energia-eletrica"
	// (id: 5a583f3e-1646-4f67-bf0f-69db4203e89e), updated weekly.
	aneelTarifasURL = "https://dadosabertos.aneel.gov.br/dataset/5a583f3e-1646-4f67-bf0f-69db4203e89e/resource/fcf2906c-7c32-4b9b-a637-054e7a5234f4/download/tarifas-homologadas-distribuidoras-energia-eletrica.csv"
)

// CSV column indices (0-based) for the ANEEL tariff dataset.
// Columns: DatGeracaoConjuntoDados;DscREH;SigAgente;NumCNPJDistribuidora;
//          DatInicioVigencia;DatFimVigencia;DscBaseTarifaria;DscSubGrupo;
//          DscModalidadeTarifaria;DscClasse;DscSubClasse;DscDetalhe;
//          NomPostoTarifario;DscUnidadeTerciaria;SigAgenteAcessante;VlrTUSD;VlrTE
const (
	colDatGeracao     = 0
	colDscREH         = 1
	colSigAgente      = 2
	colNumCNPJ        = 3
	colDatInicio      = 4
	colDatFim         = 5
	colDscBase        = 6
	colDscSubGrupo    = 7
	colDscModalidade  = 8
	colDscClasse      = 9
	colDscSubClasse   = 10
	colDscDetalhe     = 11
	colNomPosto       = 12
	colDscUnidade     = 13
	colSigAcessante   = 14
	colVlrTUSD        = 15
	colVlrTE          = 16
	numCols           = 17
)

// ANEELCollector fetches energy tariffs for electricity distributors from the ANEEL
// open data portal. The data comes from the CKAN dataset
// "tarifas-distribuidoras-energia-eletrica" and includes TE and TUSD values
// per distributor, subgroup, modality and tariff period.
type ANEELCollector struct {
	csvURL     string
	httpClient *http.Client
}

// NewANEELCollector creates an ANEEL tariff collector.
// csvURL overrides the production download URL (useful for tests); if empty the
// production URL is used.
func NewANEELCollector(csvURL string) *ANEELCollector {
	if csvURL == "" {
		csvURL = aneelTarifasURL
	}
	return &ANEELCollector{
		csvURL:     csvURL,
		httpClient: &http.Client{Timeout: 120 * time.Second}, // large file
	}
}

func (c *ANEELCollector) Source() string   { return "aneel_tarifas" }
func (c *ANEELCollector) Schedule() string { return "0 9 * * 1" }

// Collect downloads the ANEEL tariff CSV and converts each data row into a
// domain.SourceRecord. The CSV uses semicolons as delimiters, quoted fields, and
// ISO-8859-1 encoding — the stdlib csv reader handles quoted fields; we treat the
// bytes as UTF-8 since the production file is effectively Latin-1 extended ASCII
// (only Portuguese characters with diacritics that round-trip cleanly in the
// context of our Go string handling).
//
// RecordKey is built as:  <CNPJ>_<DatInicioVigencia>_<DscBaseTarifaria>_<DscSubGrupo>_<DscModalidade>_<NomPosto>_<DscUnidade>
// to produce a unique, stable identifier for each tariff line.
func (c *ANEELCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.csvURL, nil)
	if err != nil {
		return nil, fmt.Errorf("aneel_tarifas: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aneel_tarifas: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aneel_tarifas: upstream returned %d", resp.StatusCode)
	}

	// The ANEEL CSV is ISO-8859-1 encoded. Decode to UTF-8 before parsing
	// to avoid PostgreSQL "invalid byte sequence for encoding UTF8" errors.
	utf8Body := transform.NewReader(resp.Body, charmap.ISO8859_1.NewDecoder())
	records, err := parseANEELCSV(utf8Body, c.Source())
	if err != nil {
		return nil, fmt.Errorf("aneel_tarifas: parse: %w", err)
	}
	return records, nil
}

// parseANEELCSV parses the ANEEL semicolon-delimited CSV line by line.
// The file uses quoted fields (all fields wrapped in double quotes).
// Parsing is done manually to avoid loading the entire ~300k row file into memory
// at once while still handling the quoted, semicolon-delimited format.
func parseANEELCSV(body interface{ Read([]byte) (int, error) }, source string) ([]domain.SourceRecord, error) {
	scanner := bufio.NewScanner(body)
	// Increase scanner buffer for long lines (some REH descriptions can be long).
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Skip header line.
	if !scanner.Scan() {
		return nil, nil
	}

	now := time.Now().UTC()
	var records []domain.SourceRecord
	lineNum := 1

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := splitSemicolonQuoted(line)
		if len(fields) < numCols {
			// Skip malformed rows rather than aborting the entire import.
			continue
		}

		cnpj := fields[colNumCNPJ]
		datInicio := fields[colDatInicio]
		dscBase := fields[colDscBase]
		dscSubGrupo := fields[colDscSubGrupo]
		dscModalidade := fields[colDscModalidade]
		nomPosto := fields[colNomPosto]
		dscUnidade := fields[colDscUnidade]

		// Build a unique, stable record key.
		key := strings.Join([]string{
			cnpj,
			datInicio,
			dscBase,
			dscSubGrupo,
			dscModalidade,
			nomPosto,
			dscUnidade,
		}, "_")

		records = append(records, domain.SourceRecord{
			Source:    source,
			RecordKey: key,
			Data: map[string]any{
				"dat_geracao_conjunto": fields[colDatGeracao],
				"resolucao_reh":        fields[colDscREH],
				"distribuidora":        fields[colSigAgente],
				"cnpj":                 cnpj,
				"dat_inicio_vigencia":  datInicio,
				"dat_fim_vigencia":     fields[colDatFim],
				"base_tarifaria":       dscBase,
				"subgrupo":             dscSubGrupo,
				"modalidade_tarifaria": dscModalidade,
				"classe":               fields[colDscClasse],
				"subclasse":            fields[colDscSubClasse],
				"detalhe":              fields[colDscDetalhe],
				"posto_tarifario":      nomPosto,
				"unidade":              dscUnidade,
				"agente_acessante":     fields[colSigAcessante],
				"vlr_tusd":             fields[colVlrTUSD],
				"vlr_te":               fields[colVlrTE],
			},
			FetchedAt: now,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning CSV: %w", err)
	}

	return records, nil
}

// splitSemicolonQuoted splits a CSV line that uses semicolons as delimiters and
// wraps every field in double quotes. It strips the surrounding quotes from each
// field. Internal double-quote escaping ("" → ") is handled as well.
func splitSemicolonQuoted(line string) []string {
	parts := strings.Split(line, ";")
	fields := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
			p = p[1 : len(p)-1]
		}
		// Unescape doubled double-quotes.
		p = strings.ReplaceAll(p, `""`, `"`)
		fields = append(fields, p)
	}
	return fields
}
