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

const mapbiomasDefaultBaseURL = "https://plataforma.brasil.mapbiomas.org"

// mapbiomasCoverageResponse represents the MapBiomas API response for land use/cover data.
type mapbiomasCoverageResponse struct {
	Data []mapbiomasCoverage `json:"data"`
}

// mapbiomasCoverage represents a single land use/cover classification entry.
type mapbiomasCoverage struct {
	TerritoryID   int     `json:"territory_id"`
	TerritoryName string  `json:"territory_name"`
	Year          int     `json:"year"`
	ClassID       int     `json:"class_id"`
	ClassName     string  `json:"class_name"`
	AreaHa        float64 `json:"area_ha"`
	Percentage    float64 `json:"percentage"`
}

// MapBiomasCollector fetches land use/cover classification data from MapBiomas.
type MapBiomasCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewMapBiomasCollector creates a new MapBiomasCollector.
// baseURL overrides the production URL; pass "" to use the default.
func NewMapBiomasCollector(baseURL string) *MapBiomasCollector {
	if baseURL == "" {
		baseURL = mapbiomasDefaultBaseURL
	}
	return &MapBiomasCollector{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *MapBiomasCollector) Source() string   { return "mapbiomas_cobertura" }
func (c *MapBiomasCollector) Schedule() string { return "0 8 1 * *" }

// Collect fetches MapBiomas coverage data for the previous year.
func (c *MapBiomasCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	year := time.Now().Year() - 1
	url := fmt.Sprintf("%s/api/v1/coverage?territory_id=1&year=%d", c.baseURL, year)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("mapbiomas_cobertura: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mapbiomas_cobertura: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mapbiomas_cobertura: upstream returned %d", resp.StatusCode)
	}

	var raw mapbiomasCoverageResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("mapbiomas_cobertura: decode: %w", err)
	}

	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("mapbiomas_cobertura: empty response")
	}

	nowUTC := time.Now().UTC()
	records := make([]domain.SourceRecord, 0, len(raw.Data))
	for _, cov := range raw.Data {
		recordKey := fmt.Sprintf("coverage_%d_%d_%d", cov.Year, cov.TerritoryID, cov.ClassID)

		records = append(records, domain.SourceRecord{
			Source:    "mapbiomas_cobertura",
			RecordKey: recordKey,
			Data: map[string]any{
				"territory_id":   cov.TerritoryID,
				"territory_name": cov.TerritoryName,
				"year":           cov.Year,
				"class_id":       cov.ClassID,
				"class_name":     cov.ClassName,
				"area_ha":        cov.AreaHa,
				"percentage":     cov.Percentage,
			},
			FetchedAt: nowUTC,
		})
	}

	return records, nil
}
