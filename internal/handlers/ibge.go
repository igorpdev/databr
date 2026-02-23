package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

var nonDigitIBGE = regexp.MustCompile(`\D`)

// IbgeHandler handles requests for /v1/ibge/*.
type IbgeHandler struct {
	httpClient *http.Client
}

// NewIbgeHandler creates a new IbgeHandler with a default HTTP client.
func NewIbgeHandler() *IbgeHandler {
	return &IbgeHandler{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// NewIbgeHandlerWithClient creates a new IbgeHandler using the provided HTTP client.
// Useful for testing with a custom transport that redirects to a mock server.
func NewIbgeHandlerWithClient(client *http.Client) *IbgeHandler {
	return &IbgeHandler{httpClient: client}
}

// GetMunicipio handles GET /v1/ibge/municipio/{ibge}.
// Proxies to servicodados.ibge.gov.br/api/v1/localidades/municipios/{code}.
// The {ibge} param is stripped of non-digits and must be exactly 7 digits.
func (h *IbgeHandler) GetMunicipio(w http.ResponseWriter, r *http.Request) {
	rawParam := chi.URLParam(r, "ibge")
	code := nonDigitIBGE.ReplaceAllString(rawParam, "")

	if len(code) != 7 {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("Código IBGE deve ter exatamente 7 dígitos, recebido: %q", rawParam))
		return
	}

	url := fmt.Sprintf("https://servicodados.ibge.gov.br/api/v1/localidades/municipios/%s", rawParam)
	resp, err := h.httpClient.Get(url)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar IBGE Localidades: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("Município não encontrado: %s", code))
		return
	}
	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("IBGE Localidades retornou status %d", resp.StatusCode))
		return
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta IBGE Localidades: "+err.Error())
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "ibge_localidades",
		CostUSDC: "0.001",
		Data:     raw,
	})
}

// GetEstados handles GET /v1/ibge/estados.
// Proxies to servicodados.ibge.gov.br/api/v1/localidades/estados.
func (h *IbgeHandler) GetEstados(w http.ResponseWriter, r *http.Request) {
	resp, err := h.httpClient.Get("https://servicodados.ibge.gov.br/api/v1/localidades/estados")
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar IBGE Localidades: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("IBGE Localidades retornou status %d", resp.StatusCode))
		return
	}

	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta IBGE Localidades: "+err.Error())
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "ibge_localidades",
		CostUSDC: "0.001",
		Data: map[string]any{
			"estados": list,
			"total":   len(list),
		},
	})
}

// GetRegioes handles GET /v1/ibge/regioes.
// Proxies to servicodados.ibge.gov.br/api/v1/localidades/regioes.
func (h *IbgeHandler) GetRegioes(w http.ResponseWriter, r *http.Request) {
	resp, err := h.httpClient.Get("https://servicodados.ibge.gov.br/api/v1/localidades/regioes")
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar IBGE Localidades: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("IBGE Localidades retornou status %d", resp.StatusCode))
		return
	}

	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta IBGE Localidades: "+err.Error())
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "ibge_localidades",
		CostUSDC: "0.001",
		Data:     map[string]any{"regioes": list, "total": len(list)},
	})
}

// GetMunicipiosPorUF handles GET /v1/ibge/municipios/{uf}.
// Proxies to servicodados.ibge.gov.br/api/v1/localidades/estados/{uf}/municipios.
// {uf} can be the 2-letter sigla (e.g. "SP") or the numeric IBGE state code.
func (h *IbgeHandler) GetMunicipiosPorUF(w http.ResponseWriter, r *http.Request) {
	uf := chi.URLParam(r, "uf")
	if uf == "" {
		jsonError(w, http.StatusBadRequest, "UF é obrigatória")
		return
	}

	url := fmt.Sprintf("https://servicodados.ibge.gov.br/api/v1/localidades/estados/%s/municipios", uf)
	resp, err := h.httpClient.Get(url)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar IBGE Localidades: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("UF não encontrada: %s", uf))
		return
	}
	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("IBGE Localidades retornou status %d", resp.StatusCode))
		return
	}

	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta IBGE Localidades: "+err.Error())
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "ibge_localidades",
		CostUSDC: "0.001",
		Data:     map[string]any{"municipios": list, "total": len(list), "uf": uf},
	})
}

// GetCNAE handles GET /v1/ibge/cnae/{codigo}.
// Proxies to servicodados.ibge.gov.br/api/v2/cnae/subclasses/{codigo}.
func (h *IbgeHandler) GetCNAE(w http.ResponseWriter, r *http.Request) {
	codigo := chi.URLParam(r, "codigo")

	url := fmt.Sprintf("https://servicodados.ibge.gov.br/api/v2/cnae/subclasses/%s", codigo)
	resp, err := h.httpClient.Get(url)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar IBGE CNAE: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("CNAE não encontrado: %s", codigo))
		return
	}
	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("IBGE CNAE retornou status %d", resp.StatusCode))
		return
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta IBGE CNAE: "+err.Error())
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "ibge_cnae",
		CostUSDC: "0.001",
		Data:     raw,
	})
}
