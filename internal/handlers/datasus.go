package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

const dataSUSBaseURL = "https://apidadosabertos.saude.gov.br"

// DATASUSHandler handles requests for /v1/saude/datasus/*.
// On-demand proxy to the DATASUS open data API (no DB required).
type DATASUSHandler struct {
	baseURL string
	client  *http.Client
}

// NewDATASUSHandler creates a handler that proxies to the DATASUS open data API.
func NewDATASUSHandler() *DATASUSHandler {
	return &DATASUSHandler{
		baseURL: dataSUSBaseURL,
		client:  newHTTPClient(30 * time.Second),
	}
}

// GetEstabelecimento handles GET /v1/saude/estabelecimentos/{cnes}.
// Returns a single health establishment by its CNES code.
func (h *DATASUSHandler) GetEstabelecimento(w http.ResponseWriter, r *http.Request) {
	cnes := chi.URLParam(r, "cnes")
	if cnes == "" {
		jsonError(w, http.StatusBadRequest, "cnes is required")
		return
	}

	url := fmt.Sprintf("%s/cnes/estabelecimentos/%s", h.baseURL, cnes)

	var result map[string]any
	if _, err := fetchJSON(r.Context(), h.client, url, nil, &result); err != nil {
		gatewayError(w, "datasus_estabelecimentos", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datasus_cnes",
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      result,
	})
}

// GetEstabelecimentos handles GET /v1/saude/estabelecimentos?municipio=IBGE&uf=XX&limit=N.
// Searches health establishments by municipality code and/or UF.
func (h *DATASUSHandler) GetEstabelecimentos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	municipio := q.Get("municipio")
	uf := q.Get("uf")

	if municipio == "" && uf == "" {
		jsonError(w, http.StatusBadRequest, "municipio (IBGE code) or uf (state) is required")
		return
	}

	limit, _ := parsePagination(r)

	// Build upstream query with proper encoding
	qv := url.Values{}
	qv.Set("limit", strconv.Itoa(limit))
	if municipio != "" {
		qv.Set("codigo_municipio", municipio)
	}
	if uf != "" {
		qv.Set("codigo_uf", uf)
	}
	upstream := fmt.Sprintf("%s/cnes/estabelecimentos?%s", h.baseURL, qv.Encode())

	var result struct {
		Estabelecimentos []map[string]any `json:"estabelecimentos"`
	}
	if _, err := fetchJSON(r.Context(), h.client, upstream, nil, &result); err != nil {
		gatewayError(w, "datasus_estabelecimentos", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datasus_cnes",
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"estabelecimentos": result.Estabelecimentos,
			"total":            len(result.Estabelecimentos),
		},
	})
}

// proxyList proxies a paginated DATASUS list endpoint, returning items under "items" key.
func (h *DATASUSHandler) proxyList(w http.ResponseWriter, r *http.Request, path, jsonField, source string) {
	limit, offset := parsePagination(r)
	upstream := fmt.Sprintf("%s/%s?limit=%d&offset=%d", h.baseURL, path, limit, offset)

	var raw map[string]any
	if _, err := fetchJSON(r.Context(), h.client, upstream, nil, &raw); err != nil {
		gatewayError(w, source, err)
		return
	}

	var items []map[string]any
	if arr, ok := raw[jsonField].([]any); ok {
		items = make([]map[string]any, 0, len(arr))
		for _, v := range arr {
			if m, ok := v.(map[string]any); ok {
				items = append(items, m)
			}
		}
	}

	respond(w, r, domain.APIResponse{
		Source:    source,
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"items": items, "total": len(items)},
	})
}

// GetMortalidade handles GET /v1/saude/mortalidade?limit=N&offset=M.
// Proxies to DATASUS SIM (Sistema de Informação sobre Mortalidade).
func (h *DATASUSHandler) GetMortalidade(w http.ResponseWriter, r *http.Request) {
	h.proxyList(w, r, "vigilancia-e-meio-ambiente/sistema-de-informacao-sobre-mortalidade", "sim", "datasus_sim")
}

// GetNascimentos handles GET /v1/saude/nascimentos?limit=N&offset=M.
// Proxies to DATASUS SINASC (Sistema de Informação sobre Nascidos Vivos).
func (h *DATASUSHandler) GetNascimentos(w http.ResponseWriter, r *http.Request) {
	h.proxyList(w, r, "vigilancia-e-meio-ambiente/sistema-de-informacao-sobre-nascidos-vivos", "sinasc", "datasus_sinasc")
}

// GetHospitais handles GET /v1/saude/hospitais?limit=N&offset=M.
// Proxies to DATASUS hospitals and beds data (CNES).
func (h *DATASUSHandler) GetHospitais(w http.ResponseWriter, r *http.Request) {
	h.proxyList(w, r, "assistencia-a-saude/hospitais-e-leitos", "hospitais_leitos", "datasus_hospitais")
}

// GetDengue handles GET /v1/saude/dengue?limit=N&offset=M.
// Proxies to DATASUS dengue/arboviroses notification data (SINAN).
func (h *DATASUSHandler) GetDengue(w http.ResponseWriter, r *http.Request) {
	h.proxyList(w, r, "arboviroses/dengue", "parametros", "datasus_dengue")
}

// GetVacinacao handles GET /v1/saude/vacinacao/{ano}?limit=N&offset=M.
// Proxies to DATASUS PNI vaccination data for a given year (2020-2030).
func (h *DATASUSHandler) GetVacinacao(w http.ResponseWriter, r *http.Request) {
	anoStr := chi.URLParam(r, "ano")
	ano, err := strconv.Atoi(anoStr)
	maxYear := time.Now().Year() + 5
	if err != nil || ano < 2020 || ano > maxYear {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("ano must be a year between 2020 and %d", maxYear))
		return
	}

	limit, offset := parsePagination(r)
	url := fmt.Sprintf("%s/vacinacao/doses-aplicadas-pni-%d?limit=%d&offset=%d",
		h.baseURL, ano, limit, offset)

	var result struct {
		Doses []map[string]any `json:"doses_aplicadas_pni"`
	}
	if _, err := fetchJSON(r.Context(), h.client, url, nil, &result); err != nil {
		gatewayError(w, "datasus_vacinacao", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datasus_vacinacao",
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"items": result.Doses,
			"total": len(result.Doses),
		},
	})
}
