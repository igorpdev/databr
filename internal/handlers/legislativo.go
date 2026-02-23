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

	url := fmt.Sprintf("https://dadosabertos.camara.leg.br/api/v2/deputados?formato=json&itens=100&pagina=%s", pagina)
	if uf != "" {
		url += "&siglaUf=" + uf
	}
	if partido != "" {
		url += "&siglaPartido=" + partido
	}

	resp, err := h.httpClient.Get(url)
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

	url := fmt.Sprintf("https://dadosabertos.camara.leg.br/api/v2/deputados/%s?formato=json", id)
	resp, err := h.httpClient.Get(url)
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

	url := fmt.Sprintf("https://dadosabertos.camara.leg.br/api/v2/proposicoes?formato=json&itens=50&pagina=%s", pagina)
	if tipo != "" {
		url += "&siglaTipo=" + tipo
	}
	if ano != "" {
		url += "&ano=" + ano
	}
	if numero != "" {
		url += "&numero=" + numero
	}

	resp, err := h.httpClient.Get(url)
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

	url := fmt.Sprintf("https://dadosabertos.camara.leg.br/api/v2/votacoes?formato=json&itens=50&pagina=%s", pagina)
	if dataInicio != "" {
		url += "&dataInicio=" + dataInicio
	}
	if dataFim != "" {
		url += "&dataFim=" + dataFim
	}
	if orgao != "" {
		url += "&siglaOrgao=" + orgao
	}

	resp, err := h.httpClient.Get(url)
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

	url := fmt.Sprintf("https://dadosabertos.camara.leg.br/api/v2/partidos?formato=json&itens=100&pagina=%s", pagina)

	resp, err := h.httpClient.Get(url)
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

	url := fmt.Sprintf("https://legis.senado.leg.br/dadosabertos/processo?ano=%s&pagina=%s", ano, pagina)
	if sigla != "" {
		url += "&sigla=" + sigla
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
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
		// Can't decode at all — return empty list.
		respond(w, r, domain.APIResponse{
			Source:   "senado_materias",
			CostUSDC: "0.001",
			Data:     map[string]any{"materias": []any{}, "total": 0},
		})
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
