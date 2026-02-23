package handlers

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// OrcamentoHandler handles requests for /v1/orcamento/* endpoints.
// It proxies SIAFI/SIOP budget data via the Portal da Transparencia API.
type OrcamentoHandler struct {
	httpClient *http.Client
	apiKey     string
}

// NewOrcamentoHandler creates a new OrcamentoHandler with default HTTP client.
func NewOrcamentoHandler() *OrcamentoHandler {
	return &OrcamentoHandler{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		apiKey:     os.Getenv("TRANSPARENCIA_API_KEY"),
	}
}

// NewOrcamentoHandlerWithClient creates a new OrcamentoHandler with a custom HTTP client and API key.
func NewOrcamentoHandlerWithClient(client *http.Client, apiKey string) *OrcamentoHandler {
	return &OrcamentoHandler{httpClient: client, apiKey: apiKey}
}

const orcamentoBase = "https://api.portaldatransparencia.gov.br/api-de-dados"

func (h *OrcamentoHandler) headers() map[string]string {
	return map[string]string{"chave-api-dados": h.apiKey}
}

// GetDespesas handles GET /v1/orcamento/despesas?ano=YYYY&orgao=CODE[&pagina=1]
// The upstream CGU API requires at least one filter beyond ano+pagina.
func (h *OrcamentoHandler) GetDespesas(w http.ResponseWriter, r *http.Request) {
	ano := r.URL.Query().Get("ano")
	if ano == "" {
		jsonError(w, http.StatusBadRequest, "parâmetro 'ano' é obrigatório")
		return
	}
	orgao := r.URL.Query().Get("orgao")
	if orgao == "" {
		jsonError(w, http.StatusBadRequest, "parâmetro 'orgao' é obrigatório (código SIAFI do órgão superior, ex: 26000 para MEC)")
		return
	}
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}
	upURL := fmt.Sprintf("%s/despesas/por-orgao?ano=%s&pagina=%s&orgaoSuperior=%s", orcamentoBase, ano, pagina, orgao)

	var dados any
	if _, err := fetchJSON(r.Context(), h.httpClient, upURL, h.headers(), &dados); err != nil {
		gatewayError(w, "siafi_despesas", err)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    "siafi_despesas",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.001",
		Data:      map[string]any{"despesas": dados},
	})
}

// GetFuncionalProgramatica handles GET /v1/orcamento/funcional-programatica?ano=YYYY
func (h *OrcamentoHandler) GetFuncionalProgramatica(w http.ResponseWriter, r *http.Request) {
	ano := r.URL.Query().Get("ano")
	if ano == "" {
		jsonError(w, http.StatusBadRequest, "parâmetro 'ano' é obrigatório")
		return
	}
	pagina := r.URL.Query().Get("pagina")
	if pagina == "" {
		pagina = "1"
	}
	upURL := fmt.Sprintf("%s/despesas/por-funcional-programatica?ano=%s&pagina=%s", orcamentoBase, ano, pagina)
	if funcao := r.URL.Query().Get("funcao"); funcao != "" {
		upURL += "&funcao=" + funcao
	}

	var dados any
	if _, err := fetchJSON(r.Context(), h.httpClient, upURL, h.headers(), &dados); err != nil {
		gatewayError(w, "siafi_funcional", err)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    "siafi_funcional_programatica",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.001",
		Data:      map[string]any{"classificacao": dados},
	})
}

// GetDocumentos handles GET /v1/orcamento/documentos?fase=N&pagina=1
func (h *OrcamentoHandler) GetDocumentos(w http.ResponseWriter, r *http.Request) {
	upURL := orcamentoBase + "/despesas/documentos?"
	q := r.URL.Query()
	params := ""
	if fase := q.Get("fase"); fase != "" {
		params += "&fase=" + fase
	}
	if data := q.Get("dataEmissao"); data != "" {
		params += "&dataEmissao=" + data
	}
	pagina := q.Get("pagina")
	if pagina == "" {
		pagina = "1"
	}
	params += "&pagina=" + pagina
	if len(params) > 0 {
		params = params[1:] // trim leading &
	}
	upURL += params

	var dados any
	if _, err := fetchJSON(r.Context(), h.httpClient, upURL, h.headers(), &dados); err != nil {
		gatewayError(w, "siafi_documentos", err)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    "siafi_documentos",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.002",
		Data:      map[string]any{"documentos": dados},
	})
}

// GetDocumento handles GET /v1/orcamento/documento/{codigo}
func (h *OrcamentoHandler) GetDocumento(w http.ResponseWriter, r *http.Request) {
	codigo := chi.URLParam(r, "codigo")
	if codigo == "" {
		jsonError(w, http.StatusBadRequest, "código do documento é obrigatório")
		return
	}
	upURL := fmt.Sprintf("%s/despesas/documentos/%s", orcamentoBase, codigo)

	var dados any
	status, err := fetchJSON(r.Context(), h.httpClient, upURL, h.headers(), &dados)
	if err != nil {
		if status == http.StatusNotFound {
			jsonError(w, http.StatusNotFound, "documento não encontrado: "+codigo)
			return
		}
		gatewayError(w, "siafi_documento", err)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    "siafi_documento",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.001",
		Data:      map[string]any{"documento": dados},
	})
}

// GetFavorecidos handles GET /v1/orcamento/favorecidos?documento=DOC&ano=YYYY&fase=N
func (h *OrcamentoHandler) GetFavorecidos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	doc := q.Get("documento")
	ano := q.Get("ano")
	fase := q.Get("fase")
	if doc == "" || ano == "" || fase == "" {
		jsonError(w, http.StatusBadRequest, "parâmetros 'documento', 'ano' e 'fase' são obrigatórios")
		return
	}
	pagina := q.Get("pagina")
	if pagina == "" {
		pagina = "1"
	}
	upURL := fmt.Sprintf("%s/despesas/documentos-por-favorecido?codigoPessoa=%s&ano=%s&fase=%s&pagina=%s", orcamentoBase, doc, ano, fase, pagina)

	var dados any
	if _, err := fetchJSON(r.Context(), h.httpClient, upURL, h.headers(), &dados); err != nil {
		gatewayError(w, "siafi_favorecidos", err)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    "siafi_favorecidos",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.002",
		Data:      map[string]any{"favorecidos": dados},
	})
}
