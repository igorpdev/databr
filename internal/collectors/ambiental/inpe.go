// Package ambiental implements collectors for INPE environmental data services.
// Covers DETER (real-time deforestation alerts) and PRODES (annual consolidated deforestation).
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

const (
	// deterBaseURL is the Terrabrasilis WFS endpoint for DETER Amazon deforestation alerts.
	// Layer: deter-amz:deter_amz — case-sensitive, NOT deter_public.
	deterBaseURL = "https://terrabrasilis.dpi.inpe.br/geoserver/deter-amz/wfs"

	// prodesBaseURL is the Terrabrasilis OWS endpoint for PRODES annual deforestation.
	// Layer: prodes-legal-amz:yearly_deforestation
	prodesBaseURL = "https://terrabrasilis.dpi.inpe.br/geoserver/ows"
)

// wfsResponse is the GeoJSON FeatureCollection returned by Terrabrasilis WFS.
type wfsResponse struct {
	Type           string       `json:"type"`
	TotalFeatures  int          `json:"totalFeatures"`
	NumberReturned int          `json:"numberReturned"`
	Features       []wfsFeature `json:"features"`
}

type wfsFeature struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Properties map[string]any `json:"properties"`
}

// --- DETER Collector ---

// DETERCollector fetches real-time deforestation alerts from INPE DETER via Terrabrasilis WFS.
type DETERCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewDETERCollector creates a DETERCollector.
// If baseURL is empty, the production Terrabrasilis endpoint is used.
func NewDETERCollector(baseURL string) *DETERCollector {
	if baseURL == "" {
		baseURL = deterBaseURL
	}
	return &DETERCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *DETERCollector) Source() string   { return "inpe_deter" }
func (c *DETERCollector) Schedule() string { return "0 15 * * *" }

// Collect fetches the latest 500 DETER deforestation alerts from Terrabrasilis WFS.
// RecordKey is the WFS feature ID (e.g. "deter_amz.fid-abc123").
func (c *DETERCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	// When using a production URL, build the full WFS query; otherwise the test server handles it directly.
	var url string
	if strings.Contains(c.baseURL, "terrabrasilis") {
		url = fmt.Sprintf(
			"%s?service=WFS&version=2.0.0&request=GetFeature&typeName=deter-amz:deter_amz&count=500&outputFormat=application/json",
			c.baseURL,
		)
	} else {
		// Test server: use baseURL directly.
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("inpe_deter: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("inpe_deter: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inpe_deter: upstream returned %d", resp.StatusCode)
	}

	var raw wfsResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("inpe_deter: decode: %w", err)
	}
	if len(raw.Features) == 0 {
		return nil, fmt.Errorf("inpe_deter: empty response from Terrabrasilis WFS")
	}

	now := time.Now().UTC()
	records := make([]domain.SourceRecord, 0, len(raw.Features))
	for _, f := range raw.Features {
		p := f.Properties

		recordKey := f.ID
		if recordKey == "" {
			// Fallback: use gid if available
			if gid, ok := p["gid"].(string); ok && gid != "" {
				recordKey = gid
			} else {
				continue
			}
		}

		areaKm2, _ := p["areamunkm"].(float64)
		municipio, _ := p["municipality"].(string)
		estado, _ := p["uf"].(string)
		dataDeteccao, _ := p["view_date"].(string)
		classeDesmatamento, _ := p["classname"].(string)

		records = append(records, domain.SourceRecord{
			Source:    "inpe_deter",
			RecordKey: recordKey,
			Data: map[string]any{
				"area_km2":            areaKm2,
				"municipio":           municipio,
				"estado":              estado,
				"data_deteccao":       dataDeteccao,
				"classe_desmatamento": classeDesmatamento,
			},
			RawData:   p,
			FetchedAt: now,
		})
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("inpe_deter: no valid records parsed")
	}

	return records, nil
}

// --- PRODES Collector ---

// PRODESCollector fetches annual consolidated deforestation data from INPE PRODES via Terrabrasilis WFS.
type PRODESCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewPRODESCollector creates a PRODESCollector.
// If baseURL is empty, the production Terrabrasilis OWS endpoint is used.
func NewPRODESCollector(baseURL string) *PRODESCollector {
	if baseURL == "" {
		baseURL = prodesBaseURL
	}
	return &PRODESCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *PRODESCollector) Source() string   { return "inpe_prodes" }
func (c *PRODESCollector) Schedule() string { return "@monthly" }

// Collect fetches the latest 1000 PRODES yearly deforestation records from Terrabrasilis WFS.
// RecordKey is "<estado>_<ano>" to allow upsert by state+year.
func (c *PRODESCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	var url string
	if strings.Contains(c.baseURL, "terrabrasilis") {
		url = fmt.Sprintf(
			"%s?service=WFS&version=2.0.0&request=GetFeature&typeName=prodes-legal-amz:yearly_deforestation&count=1000&outputFormat=application/json",
			c.baseURL,
		)
	} else {
		// Test server: use baseURL directly.
		url = c.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("inpe_prodes: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("inpe_prodes: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inpe_prodes: upstream returned %d", resp.StatusCode)
	}

	var raw wfsResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("inpe_prodes: decode: %w", err)
	}
	if len(raw.Features) == 0 {
		return nil, fmt.Errorf("inpe_prodes: empty response from Terrabrasilis WFS")
	}

	now := time.Now().UTC()
	records := make([]domain.SourceRecord, 0, len(raw.Features))
	for _, f := range raw.Features {
		p := f.Properties

		// Normalize year — the API returns it as float64 after JSON decode.
		var ano int
		switch v := p["year"].(type) {
		case float64:
			ano = int(v)
		case int:
			ano = v
		}

		estado, _ := p["state"].(string)
		if estado == "" {
			continue
		}

		// RecordKey: "<estado>_<ano>" — unique per state per year.
		recordKey := fmt.Sprintf("%s_%d", estado, ano)

		// Use the WFS feature UUID when available as a more precise key.
		if uuid, ok := p["uuid"].(string); ok && uuid != "" {
			recordKey = uuid
		}

		areaKm2, _ := p["area_km"].(float64)

		records = append(records, domain.SourceRecord{
			Source:    "inpe_prodes",
			RecordKey: recordKey,
			Data: map[string]any{
				"area_km2": areaKm2,
				"ano":      ano,
				"estado":   estado,
			},
			RawData:   p,
			FetchedAt: now,
		})
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("inpe_prodes: no valid records parsed")
	}

	return records, nil
}
