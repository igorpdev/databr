package handlers

import (
	"fmt"
	"net/http"
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

	// Build upstream query
	upstream := fmt.Sprintf("%s/cnes/estabelecimentos?limit=%d", h.baseURL, limit)
	if municipio != "" {
		upstream += "&codigo_municipio=" + municipio
	}
	if uf != "" {
		upstream += "&codigo_uf=" + uf
	}

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

// GetMortalidade handles GET /v1/saude/mortalidade?limit=N&offset=M.
// Proxies to DATASUS SIM (Sistema de Informação sobre Mortalidade).
func (h *DATASUSHandler) GetMortalidade(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	url := fmt.Sprintf("%s/vigilancia-e-meio-ambiente/sistema-de-informacao-sobre-mortalidade?limit=%d&offset=%d",
		h.baseURL, limit, offset)

	var result struct {
		SIM []map[string]any `json:"sim"`
	}
	if _, err := fetchJSON(r.Context(), h.client, url, nil, &result); err != nil {
		gatewayError(w, "datasus_sim", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datasus_sim",
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"items": result.SIM,
			"total": len(result.SIM),
		},
	})
}

// GetNascimentos handles GET /v1/saude/nascimentos?limit=N&offset=M.
// Proxies to DATASUS SINASC (Sistema de Informação sobre Nascidos Vivos).
func (h *DATASUSHandler) GetNascimentos(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	url := fmt.Sprintf("%s/vigilancia-e-meio-ambiente/sistema-de-informacao-sobre-nascidos-vivos?limit=%d&offset=%d",
		h.baseURL, limit, offset)

	var result struct {
		SINASC []map[string]any `json:"sinasc"`
	}
	if _, err := fetchJSON(r.Context(), h.client, url, nil, &result); err != nil {
		gatewayError(w, "datasus_sinasc", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datasus_sinasc",
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"items": result.SINASC,
			"total": len(result.SINASC),
		},
	})
}

// GetHospitais handles GET /v1/saude/hospitais?limit=N&offset=M.
// Proxies to DATASUS hospitals and beds data (CNES).
func (h *DATASUSHandler) GetHospitais(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	url := fmt.Sprintf("%s/assistencia-a-saude/hospitais-e-leitos?limit=%d&offset=%d",
		h.baseURL, limit, offset)

	var result struct {
		Hospitais []map[string]any `json:"hospitais_leitos"`
	}
	if _, err := fetchJSON(r.Context(), h.client, url, nil, &result); err != nil {
		gatewayError(w, "datasus_hospitais", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datasus_hospitais",
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"items": result.Hospitais,
			"total": len(result.Hospitais),
		},
	})
}

// GetDengue handles GET /v1/saude/dengue?limit=N&offset=M.
// Proxies to DATASUS dengue/arboviroses notification data (SINAN).
func (h *DATASUSHandler) GetDengue(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	url := fmt.Sprintf("%s/arboviroses/dengue?limit=%d&offset=%d",
		h.baseURL, limit, offset)

	var result struct {
		Parametros []map[string]any `json:"parametros"`
	}
	if _, err := fetchJSON(r.Context(), h.client, url, nil, &result); err != nil {
		gatewayError(w, "datasus_dengue", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datasus_dengue",
		UpdatedAt: time.Now(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"items": result.Parametros,
			"total": len(result.Parametros),
		},
	})
}

// GetVacinacao handles GET /v1/saude/vacinacao/{ano}?limit=N&offset=M.
// Proxies to DATASUS PNI vaccination data for a given year (2020-2030).
func (h *DATASUSHandler) GetVacinacao(w http.ResponseWriter, r *http.Request) {
	anoStr := chi.URLParam(r, "ano")
	ano, err := strconv.Atoi(anoStr)
	if err != nil || ano < 2020 || ano > 2030 {
		jsonError(w, http.StatusBadRequest, "ano must be a year between 2020 and 2030")
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
