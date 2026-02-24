// Package tributario implements collectors for Brazilian tax data (IBPT, ICMS).
package tributario

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
)

const (
	sourceIBPT     = "ibpt_tributos"
	defaultIBPTURL = "https://api-ibpt.seunegocionanuvem.com.br"
)

// IBPTCollector fetches approximate tax burden data from the IBPT API.
// This is an on-demand collector — no background schedule.
type IBPTCollector struct {
	baseURL    string
	httpClient *http.Client
}

// NewIBPTCollector creates a new IBPT collector.
// baseURL defaults to the public IBPT API if empty.
func NewIBPTCollector(baseURL string) *IBPTCollector {
	if baseURL == "" {
		baseURL = defaultIBPTURL
	}
	return &IBPTCollector{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *IBPTCollector) Source() string   { return sourceIBPT }
func (c *IBPTCollector) Schedule() string { return "" }
func (c *IBPTCollector) Collect(_ context.Context) ([]domain.SourceRecord, error) {
	return nil, nil
}

// ibptResponse maps the JSON response from the IBPT API.
type ibptResponse struct {
	Codigo            string `json:"codigo"`
	Ex                string `json:"ex"`
	Tipo              int    `json:"tipo"`
	Descricao         string `json:"descricao"`
	NacionalFederal   string `json:"nacionalfederal"`
	ImportadosFederal string `json:"importadosfederal"`
	Estadual          string `json:"estadual"`
	Municipal         string `json:"municipal"`
	VigenciaInicio    string `json:"vigenciainicio"`
	VigenciaFim       string `json:"vigenciafim"`
	Versao            string `json:"versao"`
	Fonte             string `json:"fonte"`
	UF                string `json:"uf"`
}

// ibptErrorResponse maps the error JSON from the IBPT API.
type ibptErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// FetchByNCM fetches tax data for a given NCM/NBS code and UF.
// codigo should be a numeric string (1-9 digits). uf should be a 2-letter state code.
func (c *IBPTCollector) FetchByNCM(ctx context.Context, codigo, uf string) ([]domain.SourceRecord, error) {
	uf = strings.ToUpper(strings.TrimSpace(uf))
	codigo = strings.TrimSpace(codigo)

	if codigo == "" || uf == "" {
		return nil, fmt.Errorf("ibpt: codigo and uf are required")
	}
	if len(uf) != 2 {
		return nil, fmt.Errorf("ibpt: uf must be a 2-letter state code")
	}

	url := fmt.Sprintf("%s/api_ibpt.php?codigo=%s&uf=%s", c.baseURL, codigo, uf)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ibpt: build request: %w", err)
	}
	// ModSecurity on this server blocks Go's default User-Agent ("Go-http-client/2.0").
	// Setting a custom UA avoids the 406 Not Acceptable response.
	req.Header.Set("User-Agent", "DataBR/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ibpt: fetch %s/%s: %w", codigo, uf, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("ibpt: NCM %s not found in %s", codigo, uf)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ibpt: upstream returned %d for %s/%s", resp.StatusCode, codigo, uf)
	}

	// Try to decode — could be success or error response.
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("ibpt: decode response: %w", err)
	}

	// Check for error response: {"error": {"message": "...", "code": 400}}
	if errObj, ok := raw["error"]; ok {
		if errMap, ok := errObj.(map[string]any); ok {
			msg, _ := errMap["message"].(string)
			code, _ := errMap["code"].(float64)
			if int(code) == 404 {
				return nil, fmt.Errorf("ibpt: NCM %s not found in %s", codigo, uf)
			}
			return nil, fmt.Errorf("ibpt: %s", msg)
		}
	}

	// Normalize the response.
	data := normalizeIBPT(raw)
	recordKey := fmt.Sprintf("%s_%s", codigo, strings.ToLower(uf))

	record := domain.SourceRecord{
		Source:    sourceIBPT,
		RecordKey: recordKey,
		Data:      data,
		RawData:   raw,
		FetchedAt: time.Now().UTC(),
	}

	return []domain.SourceRecord{record}, nil
}

// normalizeIBPT converts the raw IBPT response into a clean data map.
func normalizeIBPT(raw map[string]any) map[string]any {
	fedNac := parseFloat(raw, "nacionalfederal")
	fedImp := parseFloat(raw, "importadosfederal")
	est := parseFloat(raw, "estadual")
	mun := parseFloat(raw, "municipal")

	tipoNum, _ := raw["tipo"].(float64)
	tipoStr := "ncm"
	if int(tipoNum) == 2 {
		tipoStr = "servico"
	}

	return map[string]any{
		"codigo":    raw["codigo"],
		"descricao": raw["descricao"],
		"tipo":      tipoStr,
		"uf":        raw["uf"],
		"aliquotas": map[string]any{
			"federal_nacional":   fedNac,
			"federal_importados": fedImp,
			"estadual":           est,
			"municipal":          mun,
			"total_nacional":     fedNac + est + mun,
			"total_importados":   fedImp + est + mun,
		},
		"vigencia": map[string]any{
			"inicio": raw["vigenciainicio"],
			"fim":    raw["vigenciafim"],
		},
		"versao": raw["versao"],
		"fonte":  raw["fonte"],
	}
}

// parseFloat extracts a float64 from a raw JSON map field (string or number).
func parseFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case string:
		f, _ := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
		return f
	case float64:
		return v
	default:
		return 0
	}
}
