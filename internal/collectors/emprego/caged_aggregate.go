package emprego

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

// cagedAgg holds running aggregation state for one (UF, CNAE secao) combination.
type cagedAgg struct {
	uf            string
	secao         string
	descricao     string
	admissoes     int
	desligamentos int
	somaSalario   float64
	countSalario  int
}

// aggregateCAGED reads CAGED semicolon-delimited CSV from r, aggregates
// by (UF, CNAE secao), and returns a single SourceRecord for the period.
// Columns are found by header name, not position, for resilience.
func aggregateCAGED(r io.Reader, periodo string) ([]domain.SourceRecord, error) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.LazyQuotes = true
	cr.ReuseRecord = true

	// Read header to find column indices
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("caged_aggregate: read header: %w", err)
	}

	colUF := -1
	colSecao := -1
	colSaldo := -1
	colSalario := -1
	for i, name := range header {
		name = strings.TrimSpace(strings.ToLower(name))
		switch {
		case name == "uf":
			colUF = i
		case strings.Contains(name, "seção") || strings.Contains(name, "secao") || name == "seção":
			colSecao = i
		case strings.Contains(name, "saldomovimenta"):
			colSaldo = i
		case name == "salário" || name == "salario":
			colSalario = i
		}
	}

	if colUF < 0 || colSecao < 0 || colSaldo < 0 {
		// Return empty result if required columns not found (graceful degradation)
		return []domain.SourceRecord{{
			Source:    "caged_emprego",
			RecordKey: periodo,
			Data:      map[string]any{"periodo": periodo, "items": []map[string]any{}, "total": 0},
			FetchedAt: time.Now().UTC(),
		}}, nil
	}

	aggs := make(map[string]*cagedAgg)

	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed rows
			continue
		}

		ufCode := ""
		if colUF < len(row) {
			ufCode = strings.TrimSpace(row[colUF])
		}
		ufSigla := ufCodeToSigla(ufCode)
		if ufSigla == "" {
			continue // Unknown UF
		}

		secao := ""
		if colSecao < len(row) {
			secao = strings.TrimSpace(row[colSecao])
		}
		if secao == "" {
			continue
		}

		key := ufSigla + "_" + secao
		agg, ok := aggs[key]
		if !ok {
			agg = &cagedAgg{
				uf:        ufSigla,
				secao:     secao,
				descricao: cnaeSecaoDescricao(secao),
			}
			aggs[key] = agg
		}

		saldo := ""
		if colSaldo < len(row) {
			saldo = strings.TrimSpace(row[colSaldo])
		}
		if saldo == "1" {
			agg.admissoes++
		} else if saldo == "-1" {
			agg.desligamentos++
		}

		if colSalario >= 0 && colSalario < len(row) {
			sal := parseBRDecimal(row[colSalario])
			if sal > 0 {
				agg.somaSalario += sal
				agg.countSalario++
			}
		}
	}

	// Convert to sorted slice
	items := make([]map[string]any, 0, len(aggs))
	for _, agg := range aggs {
		salMedio := 0.0
		if agg.countSalario > 0 {
			salMedio = agg.somaSalario / float64(agg.countSalario)
		}
		items = append(items, map[string]any{
			"uf":             agg.uf,
			"cnae_secao":     agg.secao,
			"cnae_descricao": agg.descricao,
			"admissoes":      agg.admissoes,
			"desligamentos":  agg.desligamentos,
			"saldo":          agg.admissoes - agg.desligamentos,
			"salario_medio":  salMedio,
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
		Source:    "caged_emprego",
		RecordKey: periodo,
		Data:      map[string]any{"periodo": periodo, "items": items, "total": len(items)},
		FetchedAt: time.Now().UTC(),
	}}, nil
}
