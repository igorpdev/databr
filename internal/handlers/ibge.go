package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
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

// sidraFetch fetches the latest n periods from IBGE SIDRA for the given table/variable/localidade.
// localidade should be "N1%5Ball%5D" for national or "N3%5Ball%5D" for states.
func (h *IbgeHandler) sidraFetch(ctx context.Context, tabela, variavel, localidade string, n int) ([]any, error) {
	url := fmt.Sprintf(
		"https://servicodados.ibge.gov.br/api/v3/agregados/%s/periodos/-%d/variaveis/%s?localidades=%s&view=flat",
		tabela, n, variavel, localidade,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IBGE SIDRA retornou %d para tabela %s", resp.StatusCode, tabela)
	}
	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("erro ao decodificar SIDRA: %w", err)
	}
	return list, nil
}

// GetPNAD handles GET /v1/ibge/pnad.
// Returns recent PNAD Contínua unemployment rate (table 4099, var 4099) — Brasil.
// Optional query param: n (periods, default 6).
func (h *IbgeHandler) GetPNAD(w http.ResponseWriter, r *http.Request) {
	n := 6
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 40 {
			n = v
		}
	}
	dados, err := h.sidraFetch(r.Context(), "4099", "4099", "N1%5Ball%5D", n)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, r, domain.APIResponse{
		Source:   "ibge_pnad",
		CostUSDC: "0.001",
		Data:     map[string]any{"dados": dados, "total": len(dados), "descricao": "PNAD Contínua - Taxa de desocupação (%)"},
	})
}

// GetINPC handles GET /v1/ibge/inpc.
// Returns recent INPC monthly variation (table 1736, var 44) — Brasil.
// Optional query param: n (periods, default 12).
func (h *IbgeHandler) GetINPC(w http.ResponseWriter, r *http.Request) {
	n := 12
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 60 {
			n = v
		}
	}
	dados, err := h.sidraFetch(r.Context(), "1736", "44", "N1%5Ball%5D", n)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, r, domain.APIResponse{
		Source:   "ibge_inpc",
		CostUSDC: "0.001",
		Data:     map[string]any{"dados": dados, "total": len(dados), "descricao": "INPC - Variação mensal (%)"},
	})
}

// GetPIM handles GET /v1/ibge/pim.
// Returns recent PIM-PF industrial production index (table 8888, var 12606) — Brasil.
// Optional query param: n (periods, default 12).
func (h *IbgeHandler) GetPIM(w http.ResponseWriter, r *http.Request) {
	n := 12
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 60 {
			n = v
		}
	}
	dados, err := h.sidraFetch(r.Context(), "8888", "12606", "N1%5Ball%5D", n)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, r, domain.APIResponse{
		Source:   "ibge_pim",
		CostUSDC: "0.001",
		Data:     map[string]any{"dados": dados, "total": len(dados), "descricao": "PIM-PF - Índice base fixa sem ajuste sazonal (Dez 2022=100)"},
	})
}

// GetPopulacao handles GET /v1/ibge/populacao.
// Returns population estimates by state (table 6579, var 9324) — N3 (estados).
// Optional query param: n (periods, default 3).
func (h *IbgeHandler) GetPopulacao(w http.ResponseWriter, r *http.Request) {
	n := 3
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 10 {
			n = v
		}
	}
	dados, err := h.sidraFetch(r.Context(), "6579", "9324", "N3%5Ball%5D", n)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, r, domain.APIResponse{
		Source:   "ibge_populacao",
		CostUSDC: "0.001",
		Data:     map[string]any{"dados": dados, "total": len(dados), "descricao": "Estimativa de população por estado"},
	})
}

// GetIPCA15 handles GET /v1/ibge/ipca15.
// Returns recent IPCA-15 monthly variation (table 1705, var 356) — Brasil.
// Optional query param: n (periods, default 12).
func (h *IbgeHandler) GetIPCA15(w http.ResponseWriter, r *http.Request) {
	n := 12
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 60 {
			n = v
		}
	}
	dados, err := h.sidraFetch(r.Context(), "1705", "356", "N1%5Ball%5D", n)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, r, domain.APIResponse{
		Source:   "ibge_ipca15",
		CostUSDC: "0.001",
		Data:     map[string]any{"dados": dados, "total": len(dados), "descricao": "IPCA-15 - Variação mensal (%)"},
	})
}

// GetPMC handles GET /v1/ibge/pmc.
// Returns recent PMC (Pesquisa Mensal de Comércio) retail sales volume index.
// SIDRA table 8881, variable 11709 (índice de volume de vendas do comércio varejista) — Brasil.
// Optional query param: n (periods, default 3, max 20).
func (h *IbgeHandler) GetPMC(w http.ResponseWriter, r *http.Request) {
	n := 3
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 20 {
			n = v
		}
	}
	items, err := h.sidraFetch(r.Context(), "8881", "11709", "N1%5Ball%5D", n)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, r, domain.APIResponse{
		Source:   "ibge_pmc",
		CostUSDC: "0.001",
		Data: map[string]any{
			"pmc":       items,
			"total":     len(items),
			"descricao": "PMC - Índice de volume de vendas do comércio varejista (2022=100)",
		},
	})
}

// GetPMS handles GET /v1/ibge/pms.
// Returns recent PMS (Pesquisa Mensal de Serviços) services nominal revenue index.
// SIDRA table 8162, variable 11622 (receita nominal de serviços) — Brasil.
// Optional query param: n (periods, default 3, max 20).
func (h *IbgeHandler) GetPMS(w http.ResponseWriter, r *http.Request) {
	n := 3
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 20 {
			n = v
		}
	}
	items, err := h.sidraFetch(r.Context(), "8162", "11622", "N1%5Ball%5D", n)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	respond(w, r, domain.APIResponse{
		Source:   "ibge_pms",
		CostUSDC: "0.001",
		Data: map[string]any{
			"pms":       items,
			"total":     len(items),
			"descricao": "PMS - Receita nominal de serviços (índice base 2014=100)",
		},
	})
}
