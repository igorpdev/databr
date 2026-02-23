package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"regexp"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

var onlyDigits = regexp.MustCompile(`^\d+$`)

// LegislativoHandler handles requests for /v1/legislativo/*.
type LegislativoHandler struct {
	httpClient *http.Client
}

// NewLegislativoHandler creates a new LegislativoHandler with a default HTTP client (10s timeout).
func NewLegislativoHandler() *LegislativoHandler {
	return &LegislativoHandler{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// NewLegislativoHandlerWithClient creates a new LegislativoHandler using the provided HTTP client.
// Useful for testing with a custom transport that redirects to a mock server.
func NewLegislativoHandlerWithClient(client *http.Client) *LegislativoHandler {
	return &LegislativoHandler{httpClient: client}
}

// GetDeputados handles GET /v1/legislativo/deputados.
// Optional query params: uf, partido, pagina (default "1").
func (h *LegislativoHandler) GetDeputados(w http.ResponseWriter, r *http.Request) {
	uf := r.URL.Query().Get("uf")
	partido := r.URL.Query().Get("partido")
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}

	params := neturl.Values{}
	params.Set("formato", "json")
	params.Set("itens", "100")
	params.Set("pagina", pagina)
	if uf != "" {
		params.Set("siglaUf", uf)
	}
	if partido != "" {
		params.Set("siglaPartido", partido)
	}
	apiURL := "https://dadosabertos.camara.leg.br/api/v2/deputados?" + params.Encode()

	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar Câmara dos Deputados: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}

	var body struct {
		Dados []any `json:"dados"`
		Links []any `json:"links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}

	dados := body.Dados
	if dados == nil {
		dados = []any{}
	}

	respond(w, r, domain.APIResponse{
		Source:   "camara_deputados",
		CostUSDC: "0.001",
		Data:     map[string]any{"deputados": dados, "total": len(dados)},
	})
}

// GetDeputado handles GET /v1/legislativo/deputados/{id}.
func (h *LegislativoHandler) GetDeputado(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" || !onlyDigits.MatchString(id) {
		jsonError(w, http.StatusBadRequest, "ID de deputado inválido")
		return
	}

	params := neturl.Values{}
	params.Set("formato", "json")
	apiURL := fmt.Sprintf("https://dadosabertos.camara.leg.br/api/v2/deputados/%s?%s", id, params.Encode())
	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar Câmara dos Deputados: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		jsonError(w, http.StatusNotFound, "Deputado não encontrado")
		return
	}
	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}

	var body struct {
		Dados map[string]any `json:"dados"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "camara_deputados",
		CostUSDC: "0.001",
		Data:     body.Dados,
	})
}

// GetProposicoes handles GET /v1/legislativo/proposicoes.
// Optional query params: tipo, ano, numero, pagina (default "1").
func (h *LegislativoHandler) GetProposicoes(w http.ResponseWriter, r *http.Request) {
	tipo := r.URL.Query().Get("tipo")
	ano := r.URL.Query().Get("ano")
	numero := r.URL.Query().Get("numero")
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}

	params := neturl.Values{}
	params.Set("formato", "json")
	params.Set("itens", "50")
	params.Set("pagina", pagina)
	if tipo != "" {
		params.Set("siglaTipo", tipo)
	}
	if ano != "" {
		params.Set("ano", ano)
	}
	if numero != "" {
		params.Set("numero", numero)
	}
	apiURL := "https://dadosabertos.camara.leg.br/api/v2/proposicoes?" + params.Encode()

	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar proposições da Câmara: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}

	var body struct {
		Dados []any `json:"dados"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}

	dados := body.Dados
	if dados == nil {
		dados = []any{}
	}

	respond(w, r, domain.APIResponse{
		Source:   "camara_proposicoes",
		CostUSDC: "0.001",
		Data:     map[string]any{"proposicoes": dados, "total": len(dados)},
	})
}

// GetVotacoes handles GET /v1/legislativo/votacoes.
// Optional query params: pagina (default "1"), dataInicio, dataFim, orgao (e.g. "Plenário").
func (h *LegislativoHandler) GetVotacoes(w http.ResponseWriter, r *http.Request) {
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}
	dataInicio := r.URL.Query().Get("dataInicio")
	dataFim := r.URL.Query().Get("dataFim")
	orgao := r.URL.Query().Get("orgao")

	params := neturl.Values{}
	params.Set("formato", "json")
	params.Set("itens", "50")
	params.Set("pagina", pagina)
	if dataInicio != "" {
		params.Set("dataInicio", dataInicio)
	}
	if dataFim != "" {
		params.Set("dataFim", dataFim)
	}
	if orgao != "" {
		params.Set("siglaOrgao", orgao)
	}
	apiURL := "https://dadosabertos.camara.leg.br/api/v2/votacoes?" + params.Encode()

	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar votações da Câmara: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}

	var body struct {
		Dados []any `json:"dados"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}

	dados := body.Dados
	if dados == nil {
		dados = []any{}
	}

	respond(w, r, domain.APIResponse{
		Source:   "camara_votacoes",
		CostUSDC: "0.001",
		Data:     map[string]any{"votacoes": dados, "total": len(dados)},
	})
}

// GetPartidos handles GET /v1/legislativo/partidos.
// Returns all political parties registered at the Câmara.
func (h *LegislativoHandler) GetPartidos(w http.ResponseWriter, r *http.Request) {
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}

	params := neturl.Values{}
	params.Set("formato", "json")
	params.Set("itens", "100")
	params.Set("pagina", pagina)
	apiURL := "https://dadosabertos.camara.leg.br/api/v2/partidos?" + params.Encode()

	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar partidos da Câmara: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}

	var body struct {
		Dados []any `json:"dados"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}

	dados := body.Dados
	if dados == nil {
		dados = []any{}
	}

	respond(w, r, domain.APIResponse{
		Source:   "camara_partidos",
		CostUSDC: "0.001",
		Data:     map[string]any{"partidos": dados, "total": len(dados)},
	})
}

// GetSenadores handles GET /v1/legislativo/senado/senadores.
// Returns the current list of senators from the Senado Federal.
func (h *LegislativoHandler) GetSenadores(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
		"https://legis.senado.leg.br/dadosabertos/senador/lista/atual", nil)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao criar requisição para o Senado: "+err.Error())
		return
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar Senado Federal: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Senado retornou status %d", resp.StatusCode))
		return
	}

	// Response: {"ListaParlamentarEmExercicio": {"Parlamentares": {"Parlamentar": [...]}}}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta do Senado: "+err.Error())
		return
	}

	var senadores []any
	if lista, ok := body["ListaParlamentarEmExercicio"].(map[string]any); ok {
		if parlamentares, ok := lista["Parlamentares"].(map[string]any); ok {
			if arr, ok := parlamentares["Parlamentar"].([]any); ok {
				senadores = arr
			}
		}
	}
	if senadores == nil {
		senadores = []any{}
	}

	respond(w, r, domain.APIResponse{
		Source:   "senado_senadores",
		CostUSDC: "0.001",
		Data:     map[string]any{"senadores": senadores, "total": len(senadores)},
	})
}

// GetEventos handles GET /v1/legislativo/eventos.
// Optional query params: pagina (default "1"), dataInicio, dataFim, orgao (committee sigla, e.g. "PLEN").
func (h *LegislativoHandler) GetEventos(w http.ResponseWriter, r *http.Request) {
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}
	dataInicio := r.URL.Query().Get("dataInicio")
	dataFim := r.URL.Query().Get("dataFim")
	orgao := r.URL.Query().Get("orgao")

	params := neturl.Values{}
	params.Set("formato", "json")
	params.Set("itens", "50")
	params.Set("pagina", pagina)
	if dataInicio != "" {
		params.Set("dataInicio", dataInicio)
	}
	if dataFim != "" {
		params.Set("dataFim", dataFim)
	}
	if orgao != "" {
		params.Set("siglaOrgao", orgao)
	}
	apiURL := "https://dadosabertos.camara.leg.br/api/v2/eventos?" + params.Encode()

	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar eventos da Câmara: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}

	var body struct {
		Dados []any `json:"dados"`
		Links []any `json:"links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}

	dados := body.Dados
	if dados == nil {
		dados = []any{}
	}

	respond(w, r, domain.APIResponse{
		Source:   "camara_eventos",
		CostUSDC: "0.001",
		Data:     map[string]any{"eventos": dados, "total": len(dados)},
	})
}

// GetComissoes handles GET /v1/legislativo/comissoes.
// Optional query params: pagina (default "1"), tipo (codTipoOrgao, default "2" for permanent committees).
func (h *LegislativoHandler) GetComissoes(w http.ResponseWriter, r *http.Request) {
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}
	tipo := r.URL.Query().Get("tipo")
	if tipo == "" {
		tipo = "2"
	}

	params := neturl.Values{}
	params.Set("codTipoOrgao", tipo)
	params.Set("formato", "json")
	params.Set("itens", "100")
	params.Set("pagina", pagina)
	apiURL := "https://dadosabertos.camara.leg.br/api/v2/orgaos?" + params.Encode()

	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar comissões da Câmara: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}

	var body struct {
		Dados []any `json:"dados"`
		Links []any `json:"links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}

	dados := body.Dados
	if dados == nil {
		dados = []any{}
	}

	respond(w, r, domain.APIResponse{
		Source:   "camara_comissoes",
		CostUSDC: "0.001",
		Data:     map[string]any{"comissoes": dados, "total": len(dados)},
	})
}

// GetFrentes handles GET /v1/legislativo/frentes.
// Returns parliamentary fronts (frentes parlamentares) from Câmara dos Deputados.
// Optional query params: pagina (default "1").
func (h *LegislativoHandler) GetFrentes(w http.ResponseWriter, r *http.Request) {
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}

	params := neturl.Values{}
	params.Set("formato", "json")
	params.Set("itens", "100")
	params.Set("pagina", pagina)
	apiURL := "https://dadosabertos.camara.leg.br/api/v2/frentes?" + params.Encode()

	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar frentes parlamentares: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}
	var body struct {
		Dados []any `json:"dados"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}
	dados := body.Dados
	if dados == nil {
		dados = []any{}
	}
	respond(w, r, domain.APIResponse{
		Source:   "camara_frentes",
		CostUSDC: "0.001",
		Data:     map[string]any{"frentes": dados, "total": len(dados)},
	})
}

// GetBlocos handles GET /v1/legislativo/blocos.
// Returns party blocs (blocos partidários) from Câmara dos Deputados.
// Optional query params: pagina (default "1").
func (h *LegislativoHandler) GetBlocos(w http.ResponseWriter, r *http.Request) {
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}

	params := neturl.Values{}
	params.Set("formato", "json")
	params.Set("itens", "100")
	params.Set("pagina", pagina)
	apiURL := "https://dadosabertos.camara.leg.br/api/v2/blocos?" + params.Encode()

	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar blocos partidários: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}
	var body struct {
		Dados []any `json:"dados"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}
	dados := body.Dados
	if dados == nil {
		dados = []any{}
	}
	respond(w, r, domain.APIResponse{
		Source:   "camara_blocos",
		CostUSDC: "0.001",
		Data:     map[string]any{"blocos": dados, "total": len(dados)},
	})
}

// GetDespesas handles GET /v1/legislativo/deputados/{id}/despesas.
// Returns expense reports for a specific deputy.
// Optional query params: ano (default current year), pagina (default "1").
func (h *LegislativoHandler) GetDespesas(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" || !onlyDigits.MatchString(id) {
		jsonError(w, http.StatusBadRequest, "ID de deputado inválido")
		return
	}
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}
	ano := r.URL.Query().Get("ano")
	if ano == "" {
		ano = fmt.Sprintf("%d", time.Now().Year())
	}

	params := neturl.Values{}
	params.Set("formato", "json")
	params.Set("itens", "50")
	params.Set("pagina", pagina)
	params.Set("ano", ano)
	apiURL := fmt.Sprintf(
		"https://dadosabertos.camara.leg.br/api/v2/deputados/%s/despesas?%s",
		id, params.Encode(),
	)
	resp, err := h.httpClient.Get(apiURL)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar despesas do deputado: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		jsonError(w, http.StatusNotFound, "Deputado não encontrado: "+id)
		return
	}
	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Câmara retornou status %d", resp.StatusCode))
		return
	}
	var body struct {
		Dados []any `json:"dados"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta da Câmara: "+err.Error())
		return
	}
	dados := body.Dados
	if dados == nil {
		dados = []any{}
	}
	respond(w, r, domain.APIResponse{
		Source:   "camara_despesas",
		CostUSDC: "0.001",
		Data:     map[string]any{"despesas": dados, "total": len(dados), "deputado_id": id, "ano": ano},
	})
}

// GetMateriasSenado handles GET /v1/legislativo/senado/materias.
// Optional query params: ano (default: current year), sigla, pagina (default "1").
func (h *LegislativoHandler) GetMateriasSenado(w http.ResponseWriter, r *http.Request) {
	ano := r.URL.Query().Get("ano")
	if ano == "" {
		ano = fmt.Sprintf("%d", time.Now().Year())
	}
	sigla := r.URL.Query().Get("sigla")
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}

	params := neturl.Values{}
	params.Set("ano", ano)
	params.Set("pagina", pagina)
	if sigla != "" {
		params.Set("sigla", sigla)
	}
	apiURL := "https://legis.senado.leg.br/dadosabertos/processo?" + params.Encode()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, nil)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao criar requisição para o Senado: "+err.Error())
		return
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar Senado Federal: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("Senado retornou status %d", resp.StatusCode))
		return
	}

	var list []any

	// Try to decode as plain array first.
	var rawBody any
	if err := json.NewDecoder(resp.Body).Decode(&rawBody); err != nil {
		// Can't decode at all — return 502 instead of empty 200.
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta do Senado")
		return
	}

	switch v := rawBody.(type) {
	case []any:
		// Plain JSON array.
		list = v
	case map[string]any:
		// Try nested envelope: {"Materias": {"Materia": [...]}}
		extracted := false
		if materias, ok := v["Materias"]; ok {
			if materiasMap, ok := materias.(map[string]any); ok {
				if materia, ok := materiasMap["Materia"]; ok {
					if arr, ok := materia.([]any); ok {
						list = arr
						extracted = true
					}
				}
			}
		}
		if !extracted {
			list = []any{}
		}
	default:
		list = []any{}
	}

	respond(w, r, domain.APIResponse{
		Source:   "senado_materias",
		CostUSDC: "0.001",
		Data:     map[string]any{"materias": list, "total": len(list)},
	})
}
