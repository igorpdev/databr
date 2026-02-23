package legislativo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const senadoBase = "https://legis.senado.leg.br/dadosabertos"

// SenadoCollector fetches the list of current senators from the Senado Federal API.
type SenadoCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewSenadoCollector creates a Senado collector.
// baseURL should be the Senado Dados Abertos base URL; if empty, the production URL is used.
func NewSenadoCollector(baseURL string) *SenadoCollector {
	if baseURL == "" {
		baseURL = senadoBase
	}
	return &SenadoCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *SenadoCollector) Source() string   { return "senado_senadores" }
func (c *SenadoCollector) Schedule() string { return "0 22 * * 1-5" }

// Collect fetches the list of currently active senators.
func (c *SenadoCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	url := fmt.Sprintf("%s/senador/lista/atual.json", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("senado_senadores: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("senado_senadores: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("senado_senadores: API returned %d", resp.StatusCode)
	}

	var envelope struct {
		ListaParlamentarEmExercicio struct {
			Parlamentares struct {
				Parlamentar []map[string]any `json:"Parlamentar"`
			} `json:"Parlamentares"`
		} `json:"ListaParlamentarEmExercicio"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("senado_senadores: decode: %w", err)
	}

	parlamentares := envelope.ListaParlamentarEmExercicio.Parlamentares.Parlamentar
	if len(parlamentares) == 0 {
		return nil, fmt.Errorf("senado_senadores: empty response")
	}

	records := make([]domain.SourceRecord, 0, len(parlamentares))
	for _, p := range parlamentares {
		codigo := fmt.Sprintf("%v", p["CodigoParlamentar"])
		records = append(records, domain.SourceRecord{
			Source:    "senado_senadores",
			RecordKey: codigo,
			Data:      p,
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
