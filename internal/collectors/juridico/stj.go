package juridico

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const stjDefaultBaseURL = "https://scon.stj.jus.br"

// stjResponse represents the STJ jurisprudence API response.
type stjResponse struct {
	Result []stjDecision `json:"result"`
	Total  int           `json:"total"`
}

// stjDecision represents a single STJ appellate decision.
type stjDecision struct {
	Processo    string `json:"processo"`
	Classe      string `json:"classe"`
	Numero      string `json:"numero"`
	Relator     string `json:"relator"`
	OrgaoJulg   string `json:"orgao_julgador"`
	DataJulg    string `json:"data_julgamento"`
	DataPubl    string `json:"data_publicacao"`
	Ementa      string `json:"ementa"`
	Acordao     string `json:"acordao"`
}

// STJCollector fetches recent decisions from the STJ (Superior Tribunal de Justica) API.
type STJCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewSTJCollector creates a new STJCollector.
// baseURL overrides the production URL; pass "" to use the default.
func NewSTJCollector(baseURL string) *STJCollector {
	if baseURL == "" {
		baseURL = stjDefaultBaseURL
	}
	return &STJCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *STJCollector) Source() string   { return "stj_decisoes" }
func (c *STJCollector) Schedule() string { return "0 13 * * 1-5" }

// Collect fetches STJ decisions from the last 30 days.
func (c *STJCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	now := time.Now()
	dataFim := now.Format("2006-01-02")
	dataInicio := now.AddDate(0, 0, -30).Format("2006-01-02")

	url := fmt.Sprintf(
		"%s/api/v1/jurisprudencia?data_inicio=%s&data_fim=%s&pagina=1",
		c.baseURL, dataInicio, dataFim,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("stj_decisoes: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stj_decisoes: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stj_decisoes: upstream returned %d", resp.StatusCode)
	}

	var raw stjResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("stj_decisoes: decode: %w", err)
	}

	if len(raw.Result) == 0 {
		return nil, fmt.Errorf("stj_decisoes: empty response")
	}

	nowUTC := time.Now().UTC()
	records := make([]domain.SourceRecord, 0, len(raw.Result))
	for _, d := range raw.Result {
		recordKey := d.Processo
		if recordKey == "" {
			recordKey = d.Classe + "_" + d.Numero
		}
		if recordKey == "" || recordKey == "_" {
			continue
		}

		records = append(records, domain.SourceRecord{
			Source:    "stj_decisoes",
			RecordKey: recordKey,
			Data: map[string]any{
				"processo":        d.Processo,
				"classe":          d.Classe,
				"numero":          d.Numero,
				"relator":         d.Relator,
				"orgao_julgador":  d.OrgaoJulg,
				"data_julgamento": d.DataJulg,
				"data_publicacao": d.DataPubl,
				"ementa":          d.Ementa,
				"acordao":         d.Acordao,
			},
			FetchedAt: nowUTC,
		})
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("stj_decisoes: no valid records parsed")
	}

	return records, nil
}
