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

const stfDefaultBaseURL = "https://jurisprudencia.stf.jus.br"

// stfResponse represents the STF jurisprudence API response.
type stfResponse struct {
	Result []stfDecision `json:"result"`
	Total  int           `json:"total"`
}

// stfDecision represents a single STF decision.
type stfDecision struct {
	ID          string `json:"id"`
	Nome        string `json:"nome"`
	Classe      string `json:"classe"`
	Numero      string `json:"numero"`
	Relator     string `json:"relator"`
	OrgaoJulg   string `json:"orgao_julgador"`
	Publicacao  string `json:"publicacao"`
	Julgamento  string `json:"julgamento"`
	Ementa      string `json:"ementa"`
}

// STFCollector fetches recent decisions from the STF (Supremo Tribunal Federal) API.
type STFCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewSTFCollector creates a new STFCollector.
// baseURL overrides the production URL; pass "" to use the default.
func NewSTFCollector(baseURL string) *STFCollector {
	if baseURL == "" {
		baseURL = stfDefaultBaseURL
	}
	return &STFCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *STFCollector) Source() string   { return "stf_decisoes" }
func (c *STFCollector) Schedule() string { return "@daily" }

// Collect fetches STF decisions from the last 30 days.
func (c *STFCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	now := time.Now()
	dataFim := now.Format("2006-01-02")
	dataInicio := now.AddDate(0, 0, -30).Format("2006-01-02")

	url := fmt.Sprintf(
		"%s/api/v1/jurisprudencia?q=*&data_inicio=%s&data_fim=%s&pagina=1&tamanho=20",
		c.baseURL, dataInicio, dataFim,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("stf_decisoes: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stf_decisoes: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stf_decisoes: upstream returned %d", resp.StatusCode)
	}

	var raw stfResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("stf_decisoes: decode: %w", err)
	}

	if len(raw.Result) == 0 {
		return nil, fmt.Errorf("stf_decisoes: empty response")
	}

	nowUTC := time.Now().UTC()
	records := make([]domain.SourceRecord, 0, len(raw.Result))
	for _, d := range raw.Result {
		recordKey := d.ID
		if recordKey == "" {
			recordKey = d.Classe + "_" + d.Numero
		}
		if recordKey == "" || recordKey == "_" {
			continue
		}

		records = append(records, domain.SourceRecord{
			Source:    "stf_decisoes",
			RecordKey: recordKey,
			Data: map[string]any{
				"id":              d.ID,
				"nome":            d.Nome,
				"classe":          d.Classe,
				"numero":          d.Numero,
				"relator":         d.Relator,
				"orgao_julgador":  d.OrgaoJulg,
				"publicacao":      d.Publicacao,
				"julgamento":      d.Julgamento,
				"ementa":          d.Ementa,
			},
			FetchedAt: nowUTC,
		})
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("stf_decisoes: no valid records parsed")
	}

	return records, nil
}
