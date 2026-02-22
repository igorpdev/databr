package bcb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

// CreditoCollector fetches credit indicators from BCB SGS.
// Series 20542 = total credit portfolio balance.
type CreditoCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewCreditoCollector creates a credit collector.
func NewCreditoCollector(baseURL string) *CreditoCollector {
	if baseURL == "" {
		baseURL = sgsBase
	}
	return &CreditoCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *CreditoCollector) Source() string   { return "bcb_credito" }
func (c *CreditoCollector) Schedule() string { return "@monthly" }

func (c *CreditoCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "api.bcb.gov.br") {
		url = fmt.Sprintf("%s.20542/dados/ultimos/12?formato=json", c.baseURL)
	} else {
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bcb_credito: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bcb_credito: fetch: %w", err)
	}
	defer resp.Body.Close()

	// OLINDA returns {"value": [...]}
	var container struct {
		Value []map[string]any `json:"value"`
	}
	// Try OLINDA format first
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("bcb_credito: decode: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(container.Value))
	for _, entry := range container.Value {
		data, _ := entry["Data"].(string)
		valor, _ := entry["Valor"].(string)
		if data == "" {
			// Try SGS format (lowercase)
			data, _ = entry["data"].(string)
			valor, _ = entry["valor"].(string)
		}
		if data == "" {
			continue
		}
		records = append(records, domain.SourceRecord{
			Source:    "bcb_credito",
			RecordKey: data,
			Data:      map[string]any{"data": data, "valor_bilhoes_brl": valor},
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}

// ReservasCollector fetches Brazil's international reserves from BCB SGS series 3546.
// (3546 is actually IGP-M; the correct series for reservas is 13621 but for demo purposes we use a working one)
type ReservasCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewReservasCollector creates an international reserves collector.
func NewReservasCollector(baseURL string) *ReservasCollector {
	if baseURL == "" {
		baseURL = sgsBase
	}
	return &ReservasCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *ReservasCollector) Source() string   { return "bcb_reservas" }
func (c *ReservasCollector) Schedule() string { return "@daily" }

func (c *ReservasCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "api.bcb.gov.br") {
		url = fmt.Sprintf("%s.13621/dados/ultimos/12?formato=json", c.baseURL)
	} else {
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bcb_reservas: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bcb_reservas: fetch: %w", err)
	}
	defer resp.Body.Close()

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bcb_reservas: decode: %w", err)
	}

	records := make([]domain.SourceRecord, 0, len(raw))
	for _, entry := range raw {
		date, _ := entry["data"].(string)
		valor, _ := entry["valor"].(string)
		if date == "" {
			continue
		}
		records = append(records, domain.SourceRecord{
			Source:    "bcb_reservas",
			RecordKey: date,
			Data:      map[string]any{"data": date, "valor_bilhoes_usd": valor},
			FetchedAt: time.Now().UTC(),
		})
	}
	return records, nil
}
