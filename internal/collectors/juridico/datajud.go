// Package juridico implements the DataJud CNJ collector for judicial processes.
// API: https://api-publica.datajud.cnj.jus.br/
// Auth: Authorization: APIKey {key} — free public key from https://datajud-wiki.cnj.jus.br
package juridico

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

// ParseError indicates an invalid CNJ number or unsupported tribunal routing.
// Handlers use errors.As to distinguish parse errors (400) from network errors (502).
type ParseError struct {
	Msg string
}

func (e *ParseError) Error() string { return e.Msg }

const datajudBase = "https://api-publica.datajud.cnj.jus.br"

// allTribunais contains all 91 DataJud Elasticsearch indices:
// 4 superiores + 6 federal + 27 estadual + 24 trabalho + 27 eleitoral + 3 militar estadual.
var allTribunais = []string{
	// Superiores (4)
	"api_publica_stj", "api_publica_tst", "api_publica_tse", "api_publica_stm",
	// Federal (6)
	"api_publica_trf1", "api_publica_trf2", "api_publica_trf3",
	"api_publica_trf4", "api_publica_trf5", "api_publica_trf6",
	// Estadual (27)
	"api_publica_tjac", "api_publica_tjal", "api_publica_tjam", "api_publica_tjap",
	"api_publica_tjba", "api_publica_tjce", "api_publica_tjdft", "api_publica_tjes",
	"api_publica_tjgo", "api_publica_tjma", "api_publica_tjmg", "api_publica_tjms",
	"api_publica_tjmt", "api_publica_tjpa", "api_publica_tjpb", "api_publica_tjpe",
	"api_publica_tjpi", "api_publica_tjpr", "api_publica_tjrj", "api_publica_tjrn",
	"api_publica_tjro", "api_publica_tjrr", "api_publica_tjrs", "api_publica_tjsc",
	"api_publica_tjse", "api_publica_tjsp", "api_publica_tjto",
	// Trabalho (24)
	"api_publica_trt1", "api_publica_trt2", "api_publica_trt3", "api_publica_trt4",
	"api_publica_trt5", "api_publica_trt6", "api_publica_trt7", "api_publica_trt8",
	"api_publica_trt9", "api_publica_trt10", "api_publica_trt11", "api_publica_trt12",
	"api_publica_trt13", "api_publica_trt14", "api_publica_trt15", "api_publica_trt16",
	"api_publica_trt17", "api_publica_trt18", "api_publica_trt19", "api_publica_trt20",
	"api_publica_trt21", "api_publica_trt22", "api_publica_trt23", "api_publica_trt24",
	// Eleitoral (27)
	"api_publica_tre-ac", "api_publica_tre-al", "api_publica_tre-am", "api_publica_tre-ap",
	"api_publica_tre-ba", "api_publica_tre-ce", "api_publica_tre-dft", "api_publica_tre-es",
	"api_publica_tre-go", "api_publica_tre-ma", "api_publica_tre-mg", "api_publica_tre-ms",
	"api_publica_tre-mt", "api_publica_tre-pa", "api_publica_tre-pb", "api_publica_tre-pe",
	"api_publica_tre-pi", "api_publica_tre-pr", "api_publica_tre-rj", "api_publica_tre-rn",
	"api_publica_tre-ro", "api_publica_tre-rr", "api_publica_tre-rs", "api_publica_tre-sc",
	"api_publica_tre-se", "api_publica_tre-sp", "api_publica_tre-to",
	// Militar estadual (3)
	"api_publica_tjmmg", "api_publica_tjmrs", "api_publica_tjmsp",
}

// trCodeToUF maps CNJ tribunal region codes (1–27, int keys) to lowercase UF abbreviations.
// Used for justiça estadual (J=8), eleitoral (J=6), and militar estadual (J=7).
var trCodeToUF = map[int]string{
	1: "ac", 2: "al", 3: "ap", 4: "am", 5: "ba", 6: "ce", 7: "dft",
	8: "es", 9: "go", 10: "ma", 11: "mt", 12: "ms", 13: "mg",
	14: "pa", 15: "pb", 16: "pe", 17: "pi", 18: "pr", 19: "rj",
	20: "rn", 21: "rs", 22: "ro", 23: "rr", 24: "sc", 25: "se",
	26: "sp", 27: "to",
}

// militarEstadualUFs are the UFs that have military state tribunals (TJMMG, TJMRS, TJMSP).
var militarEstadualUFs = map[string]bool{
	"mg": true, "rs": true, "sp": true,
}

// searchTribunais is the high-volume subset (23 courts) used for fan-out CPF/CNPJ searches.
// Searching all 91 tribunals sequentially is too slow (~22+ min worst case at 15s timeout each).
// SearchByNumber still routes to any of the 91 via CNJ number parsing.
var searchTribunais = []string{
	// Superiores (4)
	"api_publica_stj", "api_publica_tst", "api_publica_tse", "api_publica_stm",
	// Federal (6)
	"api_publica_trf1", "api_publica_trf2", "api_publica_trf3",
	"api_publica_trf4", "api_publica_trf5", "api_publica_trf6",
	// Top 13 estaduais by case volume
	"api_publica_tjsp", "api_publica_tjrj", "api_publica_tjmg", "api_publica_tjrs",
	"api_publica_tjpr", "api_publica_tjba", "api_publica_tjpe", "api_publica_tjce",
	"api_publica_tjgo", "api_publica_tjdft", "api_publica_tjes", "api_publica_tjsc",
	"api_publica_tjma",
}

// ParseCNJNumber parses a CNJ unified process number and returns the DataJud
// Elasticsearch index name and the clean 20-digit number.
// Format: NNNNNNN-DD.AAAA.J.TR.OOOO (25 chars formatted, 20 digits unformatted).
// J = justiça, TR = tribunal region.
func ParseCNJNumber(numero string) (index, cleanNum string, err error) {
	// Strip formatting characters (hyphens and dots)
	clean := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, numero)

	if len(clean) != 20 {
		return "", "", &ParseError{Msg: fmt.Sprintf("invalid CNJ number: expected 20 digits, got %d from %q", len(clean), numero)}
	}

	// CNJ number layout (20 digits): NNNNNNNDDAAAAJTROOOO
	//   positions 0-6:  NNNNNNN (sequence)
	//   positions 7-8:  DD (check digits)
	//   positions 9-12: AAAA (year)
	//   position  13:   J (justiça)
	//   positions 14-15: TR (tribunal region, 2 digits)
	//   positions 16-19: OOOO (origin)

	j, err := strconv.Atoi(string(clean[13]))
	if err != nil {
		return "", "", &ParseError{Msg: fmt.Sprintf("invalid justice branch digit: %v", err)}
	}
	tr, err := strconv.Atoi(clean[14:16])
	if err != nil {
		return "", "", &ParseError{Msg: fmt.Sprintf("invalid tribunal region: %v", err)}
	}

	var idx string
	switch j {
	case 1: // STF (Supremo Tribunal Federal) — not available in DataJud
		return "", "", &ParseError{Msg: "STF (J=1) is not available in DataJud"}
	case 2: // CNJ (Conselho Nacional de Justiça) — not a tribunal with cases
		return "", "", &ParseError{Msg: "CNJ (J=2) does not have public processes in DataJud"}
	case 3: // STJ (Superior Tribunal de Justiça)
		idx = "api_publica_stj"
	case 4: // Justiça Federal (TRFs)
		if tr == 0 {
			return "", "", &ParseError{Msg: "invalid TR=00 for federal justice (J=4)"}
		}
		idx = fmt.Sprintf("api_publica_trf%d", tr)
	case 5: // Justiça do Trabalho
		if tr == 0 {
			idx = "api_publica_tst" // TR=00 is TST itself (superior court)
		} else {
			idx = fmt.Sprintf("api_publica_trt%d", tr)
		}
	case 6: // Justiça Eleitoral
		if tr == 0 {
			idx = "api_publica_tse" // TR=00 is TSE itself (superior court)
		} else {
			uf, ok := trCodeToUF[tr]
			if !ok {
				return "", "", &ParseError{Msg: fmt.Sprintf("unknown TR code %d for electoral justice", tr)}
			}
			idx = "api_publica_tre-" + uf
		}
	case 7: // Justiça Militar da União (STM)
		idx = "api_publica_stm"
	case 8: // Justiça Estadual
		uf, ok := trCodeToUF[tr]
		if !ok {
			return "", "", &ParseError{Msg: fmt.Sprintf("unknown TR code %d for state justice", tr)}
		}
		idx = "api_publica_tj" + uf
	case 9: // Justiça Militar Estadual
		uf, ok := trCodeToUF[tr]
		if !ok {
			return "", "", &ParseError{Msg: fmt.Sprintf("unknown TR code %d for state military justice", tr)}
		}
		if !militarEstadualUFs[uf] {
			return "", "", &ParseError{Msg: fmt.Sprintf("no military state tribunal for UF %q (TR=%d)", uf, tr)}
		}
		idx = "api_publica_tjm" + uf
	default:
		return "", "", &ParseError{Msg: fmt.Sprintf("unknown justice branch J=%d", j)}
	}

	return idx, clean, nil
}

// DataJudCollector fetches judicial process data from DataJud CNJ.
type DataJudCollector struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewDataJudCollector creates a DataJud collector.
// apiKey should be read from DATAJUD_API_KEY env var.
func NewDataJudCollector(baseURL, apiKey string) *DataJudCollector {
	if baseURL == "" {
		baseURL = datajudBase
	}
	return &DataJudCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *DataJudCollector) Source() string   { return "datajud_cnj" }
func (c *DataJudCollector) Schedule() string { return "@daily" }

// Collect is a no-op — DataJud is queried on-demand by document.
func (c *DataJudCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	return nil, nil
}

// Search queries DataJud for processes linked to the given CPF (11 digits) or CNPJ (14 digits).
// Searches across the configured list of major tribunals, stopping at 50 results.
func (c *DataJudCollector) Search(ctx context.Context, documento string) ([]domain.SourceRecord, error) {
	// Normalize: keep digits only
	normalized := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, documento)

	fieldName := "CPF"
	if len(normalized) == 14 {
		fieldName = "CNPJ"
	}

	var allRecords []domain.SourceRecord
	for _, tribunal := range searchTribunais {
		records, err := c.searchTribunal(ctx, tribunal, fieldName, normalized)
		if err != nil {
			// Non-fatal: one tribunal failing shouldn't block results from others
			continue
		}
		allRecords = append(allRecords, records...)
		if len(allRecords) >= 50 {
			break
		}
	}
	return allRecords, nil
}

func (c *DataJudCollector) searchTribunal(ctx context.Context, tribunal, fieldName, doc string) ([]domain.SourceRecord, error) {
	body := map[string]any{
		"size": 5,
		"query": map[string]any{
			"nested": map[string]any{
				"path": "partes",
				"query": map[string]any{
					"match": map[string]any{
						"partes." + fieldName: doc,
					},
				},
			},
		},
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("datajud: marshal body: %w", err)
	}

	var reqURL string
	if strings.Contains(c.baseURL, "datajud.cnj.jus.br") {
		reqURL = fmt.Sprintf("%s/%s/_search", c.baseURL, tribunal)
	} else {
		// Test server: use baseURL directly
		reqURL = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "APIKey "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("datajud: %s returned %d", tribunal, resp.StatusCode)
	}

	var raw struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	records := make([]domain.SourceRecord, 0, len(raw.Hits.Hits))
	for i, hit := range raw.Hits.Hits {
		numProcesso, _ := hit.Source["numeroProcesso"].(string)
		if numProcesso == "" {
			numProcesso = fmt.Sprintf("%s_%d", tribunal, i)
		}
		records = append(records, domain.SourceRecord{
			Source:    "datajud_cnj",
			RecordKey: numProcesso,
			Data:      hit.Source,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}

// SearchByNumber parses a CNJ process number and queries the single matching tribunal.
// Unlike Search (which fans out across all tribunals for a document), this routes to
// exactly one tribunal based on the J and TR fields encoded in the CNJ number.
func (c *DataJudCollector) SearchByNumber(ctx context.Context, numero string) ([]domain.SourceRecord, error) {
	index, cleanNum, err := ParseCNJNumber(numero)
	if err != nil {
		return nil, fmt.Errorf("datajud: %w", err)
	}

	body := map[string]any{
		"size": 1,
		"query": map[string]any{
			"match": map[string]any{
				"numeroProcesso": cleanNum,
			},
		},
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("datajud: marshal body: %w", err)
	}

	var reqURL string
	if strings.Contains(c.baseURL, "datajud.cnj.jus.br") {
		reqURL = fmt.Sprintf("%s/%s/_search", c.baseURL, index)
	} else {
		// Test server: use baseURL directly
		reqURL = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "APIKey "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("datajud: %s returned %d", index, resp.StatusCode)
	}

	var raw struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	records := make([]domain.SourceRecord, 0, len(raw.Hits.Hits))
	for i, hit := range raw.Hits.Hits {
		numProcesso, _ := hit.Source["numeroProcesso"].(string)
		if numProcesso == "" {
			numProcesso = fmt.Sprintf("%s_%d", index, i)
		}
		records = append(records, domain.SourceRecord{
			Source:    "datajud_cnj",
			RecordKey: numProcesso,
			Data:      hit.Source,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
