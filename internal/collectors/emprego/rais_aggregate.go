package emprego

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

// raisAgg holds running aggregation state for one (UF, CNAE secao) combination.
type raisAgg struct {
	uf        string
	secao     string
	descricao string
	total     int
	ativos    int
	somaRem   float64
	countRem  int
}

// aggregateRAIS reads RAIS comma-delimited CSV from r, aggregates
// by (UF, CNAE secao), and returns a single SourceRecord for the year.
//
// RAIS format differences from CAGED:
//   - Delimiter: comma (not semicolon)
//   - Encoding: Latin-1 (caller must decode before passing reader)
//   - Headers: quoted, 61 columns
//   - Columns found by substring match (not exact name) due to encoding artifacts
//
// Key columns:
//   - "CNAE 2.0 Classe" (not subclasse): 4-digit CNAE code → divisao → secao
//   - "Ind Vínculo Ativo 31/12": "1" = active bond on Dec 31
//   - "Município Trab": IBGE municipality code, first 2 digits = UF
//   - "Vl Rem Média Nom": average nominal remuneration (US decimal format)
func aggregateRAIS(r io.Reader, ano int) ([]domain.SourceRecord, error) {
	cr := csv.NewReader(r)
	cr.LazyQuotes = true
	cr.ReuseRecord = true
	cr.FieldsPerRecord = -1 // variable fields OK

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("rais_aggregate: read header: %w", err)
	}

	colCNAE := -1
	colAtivo := -1
	colMunic := -1
	colRem := -1

	for i, name := range header {
		lower := strings.ToLower(strings.Trim(name, "\" "))
		switch {
		case colCNAE < 0 && strings.Contains(lower, "cnae 2.0 classe") && !strings.Contains(lower, "sub"):
			colCNAE = i
		case colAtivo < 0 && (strings.Contains(lower, "ativo 31/12") || (strings.Contains(lower, "ativo") && strings.Contains(lower, "31"))):
			colAtivo = i
		case colMunic < 0 && strings.Contains(lower, "munic") && strings.Contains(lower, "trab"):
			colMunic = i
		case colRem < 0 && strings.Contains(lower, "rem") && strings.Contains(lower, "dia nom"):
			colRem = i
		}
	}

	emptyResult := []domain.SourceRecord{{
		Source:    "rais_emprego",
		RecordKey: strconv.Itoa(ano),
		Data:      map[string]any{"ano": ano, "items": []map[string]any{}, "total": 0},
		FetchedAt: time.Now().UTC(),
	}}

	if colCNAE < 0 || colMunic < 0 {
		return emptyResult, nil
	}

	aggs := make(map[string]*raisAgg)

	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}

		// Extract UF from municipality code (first 2 digits)
		municCode := ""
		if colMunic < len(row) {
			municCode = strings.TrimSpace(row[colMunic])
		}
		if len(municCode) < 2 {
			continue
		}
		ufCode := municCode[:2]
		ufSigla := ufCodeToSigla(ufCode)
		if ufSigla == "" {
			continue // unknown UF (e.g. "99")
		}

		// Extract CNAE section from CNAE 2.0 Classe code
		cnaeRaw := ""
		if colCNAE < len(row) {
			cnaeRaw = strings.TrimSpace(row[colCNAE])
		}
		// Pad to at least 4 digits for divisao lookup
		for len(cnaeRaw) < 4 {
			cnaeRaw = "0" + cnaeRaw
		}
		secao, descricao := cnaeDivisaoToSecao(cnaeRaw)
		if secao == "" {
			continue
		}

		key := ufSigla + "_" + secao
		agg, ok := aggs[key]
		if !ok {
			agg = &raisAgg{
				uf:        ufSigla,
				secao:     secao,
				descricao: descricao,
			}
			aggs[key] = agg
		}

		agg.total++

		// Count active bonds (Dec 31)
		if colAtivo >= 0 && colAtivo < len(row) && strings.TrimSpace(row[colAtivo]) == "1" {
			agg.ativos++
		}

		// Sum remuneration for average calculation
		if colRem >= 0 && colRem < len(row) {
			rem := parseBRDecimal(row[colRem])
			if rem > 0 {
				agg.somaRem += rem
				agg.countRem++
			}
		}
	}

	// Convert to sorted slice
	items := make([]map[string]any, 0, len(aggs))
	for _, agg := range aggs {
		remMedia := 0.0
		if agg.countRem > 0 {
			remMedia = agg.somaRem / float64(agg.countRem)
		}
		items = append(items, map[string]any{
			"uf":                agg.uf,
			"cnae_secao":        agg.secao,
			"cnae_descricao":    agg.descricao,
			"vinculos_total":    agg.total,
			"ativos_dez31":      agg.ativos,
			"remuneracao_media": remMedia,
		})
	}

	// Sort by UF then CNAE section for deterministic output
	sort.Slice(items, func(i, j int) bool {
		if items[i]["uf"].(string) != items[j]["uf"].(string) {
			return items[i]["uf"].(string) < items[j]["uf"].(string)
		}
		return items[i]["cnae_secao"].(string) < items[j]["cnae_secao"].(string)
	})

	return []domain.SourceRecord{{
		Source:    "rais_emprego",
		RecordKey: strconv.Itoa(ano),
		Data:      map[string]any{"ano": ano, "items": items, "total": len(items)},
		FetchedAt: time.Now().UTC(),
	}}, nil
}
