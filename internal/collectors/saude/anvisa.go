// Package saude implements collectors for Brazilian public health data sources.
package saude

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const (
	anvisaDefaultURL = "https://dados.anvisa.gov.br/dados/DADOS_ABERTOS_MEDICAMENTOS.csv"
)

// anvisaCSVColumns defines the expected column indices in the ANVISA CSV.
// Header: TIPO_PRODUTO;NOME_PRODUTO;DATA_FINALIZACAO_PROCESSO;CATEGORIA_REGULATORIA;
//
//	NUMERO_REGISTRO_PRODUTO;DATA_VENCIMENTO_REGISTRO;NUMERO_PROCESSO;
//	CLASSE_TERAPEUTICA;EMPRESA_DETENTORA_REGISTRO;SITUACAO_REGISTRO;PRINCIPIO_ATIVO
const (
	colTipoProduto          = 0
	colNomeProduto          = 1
	colDataFinalizacao      = 2
	colCategoriaRegulatoria = 3
	colNumeroRegistro       = 4
	colDataVencimento       = 5
	colNumeroProcesso       = 6
	colClasseTerapeutica    = 7
	colEmpresaDetentora     = 8
	colSituacaoRegistro     = 9
	colPrincipioAtivo       = 10
)

// activeStatuses are the SITUACAO_REGISTRO values kept (others are discarded).
var activeStatuses = map[string]bool{
	"ATIVO":  true,
	"VALIDO": true,
	// Handle accented version that may appear in UTF-8 decoded data.
	"VÁLIDO": true,
}

// AnvisaCollector fetches the ANVISA open data CSV of registered medicamentos.
type AnvisaCollector struct {
	csvURL     string
	httpClient *http.Client
}

// NewAnvisaCollector creates a new AnvisaCollector.
// csvURL overrides the production ANVISA CSV URL; pass "" to use the default.
func NewAnvisaCollector(csvURL string) *AnvisaCollector {
	if csvURL == "" {
		csvURL = anvisaDefaultURL
	}
	return &AnvisaCollector{
		csvURL:     csvURL,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *AnvisaCollector) Source() string   { return "anvisa_medicamentos" }
func (c *AnvisaCollector) Schedule() string { return "@monthly" }

// Collect downloads the ANVISA CSV, filters active medicamentos, and returns one
// SourceRecord per active registration. The file is ISO-8859-1 encoded and uses
// semicolons as field separators.
func (c *AnvisaCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.csvURL, nil)
	if err != nil {
		return nil, fmt.Errorf("anvisa_medicamentos: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anvisa_medicamentos: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anvisa_medicamentos: upstream returned %d", resp.StatusCode)
	}

	// The file is ISO-8859-1 (Latin-1). Decode to UTF-8 before CSV parsing.
	utf8Reader := transform.NewReader(resp.Body, charmap.ISO8859_1.NewDecoder())

	r := csv.NewReader(utf8Reader)
	r.Comma = ';'
	r.LazyQuotes = true      // some fields have unbalanced quotes
	r.TrimLeadingSpace = true

	// Read and validate header row.
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("anvisa_medicamentos: read header: %w", err)
	}
	if len(header) < colPrincipioAtivo+1 {
		return nil, fmt.Errorf("anvisa_medicamentos: unexpected column count %d in header", len(header))
	}

	now := time.Now().UTC()
	var records []domain.SourceRecord

	for {
		row, err := r.Read()
		if err != nil {
			// io.EOF ends iteration; other errors are real problems.
			if isEOF(err) {
				break
			}
			// Skip malformed rows without aborting.
			continue
		}
		if len(row) <= colPrincipioAtivo {
			continue
		}

		situacao := strings.TrimSpace(strings.Trim(row[colSituacaoRegistro], `"`))
		if !activeStatuses[situacao] {
			continue
		}

		numero := strings.TrimSpace(strings.Trim(row[colNumeroRegistro], `"`))
		if numero == "" {
			continue
		}

		// Parse EMPRESA_DETENTORA_REGISTRO — format: "CNPJ - NOME EMPRESA"
		empresa := strings.TrimSpace(strings.Trim(row[colEmpresaDetentora], `"`))
		cnpjEmpresa := ""
		nomeEmpresa := empresa
		if idx := strings.Index(empresa, " - "); idx != -1 {
			cnpjEmpresa = strings.TrimSpace(empresa[:idx])
			nomeEmpresa = strings.TrimSpace(empresa[idx+3:])
		}

		records = append(records, domain.SourceRecord{
			Source:    "anvisa_medicamentos",
			RecordKey: numero,
			Data: map[string]any{
				"produto":             strings.TrimSpace(strings.Trim(row[colNomeProduto], `"`)),
				"tipo":                strings.TrimSpace(strings.Trim(row[colTipoProduto], `"`)),
				"categoria":           strings.TrimSpace(strings.Trim(row[colCategoriaRegulatoria], `"`)),
				"numero_registro":     numero,
				"data_vencimento":     strings.TrimSpace(strings.Trim(row[colDataVencimento], `"`)),
				"data_finalizacao":    strings.TrimSpace(strings.Trim(row[colDataFinalizacao], `"`)),
				"numero_processo":     strings.TrimSpace(strings.Trim(row[colNumeroProcesso], `"`)),
				"classe_terapeutica":  strings.TrimSpace(strings.Trim(row[colClasseTerapeutica], `"`)),
				"empresa":             nomeEmpresa,
				"cnpj":                cnpjEmpresa,
				"situacao":            situacao,
				"principio_ativo":     strings.TrimSpace(strings.Trim(row[colPrincipioAtivo], `"`)),
			},
			FetchedAt: now,
		})
	}

	return records, nil
}

// isEOF reports whether err signals end of file. We use this instead of comparing
// to io.EOF directly so we handle csv.ErrFieldCount-wrapped EOFs as well.
func isEOF(err error) bool {
	return err != nil && (err.Error() == "EOF" || strings.HasSuffix(err.Error(), "EOF"))
}
