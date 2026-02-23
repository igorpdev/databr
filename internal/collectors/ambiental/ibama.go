package ambiental

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const ibamaDefaultBaseURL = "https://dados.ibama.gov.br"

// ibamaResponse represents the IBAMA embargo API response.
type ibamaResponse struct {
	Data  []ibamaEmbargo `json:"data"`
	Total int            `json:"total"`
}

// ibamaEmbargo represents a single IBAMA embargo record.
type ibamaEmbargo struct {
	ID            string `json:"id"`
	SEI           string `json:"sei"`
	CPFCNPJInfr   string `json:"cpf_cnpj_infrator"`
	NomeInfrator  string `json:"nome_infrator"`
	Municipio     string `json:"municipio"`
	UF            string `json:"uf"`
	DescInfracao  string `json:"descricao_infracao"`
	DataEmbargo   string `json:"data_embargo"`
	AreaEmbargada float64 `json:"area_embargada_ha"`
	Status        string `json:"status"`
	Bioma         string `json:"bioma"`
}

// IBAMACollector fetches embargo records from the IBAMA data source.
// Note: Direct access to dadosabertos.ibama.gov.br is blocked by Cloudflare.
// This collector uses a mirror or Base dos Dados CSV endpoint.
type IBAMACollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewIBAMACollector creates a new IBAMACollector.
// baseURL overrides the production URL; pass "" to use the default.
func NewIBAMACollector(baseURL string) *IBAMACollector {
	if baseURL == "" {
		baseURL = ibamaDefaultBaseURL
	}
	return &IBAMACollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *IBAMACollector) Source() string   { return "ibama_embargos" }
func (c *IBAMACollector) Schedule() string { return "0 8 * * 1" }

// Collect fetches IBAMA embargo data.
func (c *IBAMACollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	url := fmt.Sprintf("%s/api/embargos?page=1&size=100", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ibama_embargos: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ibama_embargos: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ibama_embargos: upstream returned %d", resp.StatusCode)
	}

	var raw ibamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("ibama_embargos: decode: %w", err)
	}

	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("ibama_embargos: empty response")
	}

	nowUTC := time.Now().UTC()
	records := make([]domain.SourceRecord, 0, len(raw.Data))
	for _, e := range raw.Data {
		recordKey := e.ID
		if recordKey == "" {
			recordKey = e.SEI
		}
		if recordKey == "" {
			continue
		}

		records = append(records, domain.SourceRecord{
			Source:    "ibama_embargos",
			RecordKey: recordKey,
			Data: map[string]any{
				"id":                e.ID,
				"sei":               e.SEI,
				"cpf_cnpj_infrator": e.CPFCNPJInfr,
				"nome_infrator":     e.NomeInfrator,
				"municipio":         e.Municipio,
				"uf":                e.UF,
				"descricao_infracao": e.DescInfracao,
				"data_embargo":      e.DataEmbargo,
				"area_embargada_ha": e.AreaEmbargada,
				"status":            e.Status,
				"bioma":             e.Bioma,
			},
			FetchedAt: nowUTC,
		})
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("ibama_embargos: no valid records parsed")
	}

	return records, nil
}
