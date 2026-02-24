package handlers

import (
	"net/http"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
)

// CaseStudy represents an AI agent use case for the DataBR API.
type CaseStudy struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	EndpointsUsed []string `json:"endpoints_used"`
	EstimatedCost string   `json:"estimated_cost_usdc"`
	Workflow      string   `json:"workflow"`
	AgentType     string   `json:"agent_type"`
}

var caseStudies = []CaseStudy{
	{
		ID:            "compliance-check",
		Title:         "Verificação de Compliance Empresarial",
		Description:   "Verificar CNPJ contra sanções CEIS/CNEP/CEPIM, processos judiciais e situação cadastral",
		EndpointsUsed: []string{"/v1/empresas/{cnpj}", "/v1/compliance/{cnpj}", "/v1/judicial/processos/{doc}"},
		EstimatedCost: "0.028",
		Workflow:      "1. Buscar dados da empresa → 2. Verificar sanções CEIS/CNEP/CEPIM → 3. Buscar processos judiciais por CNPJ",
		AgentType:     "compliance",
	},
	{
		ID:            "due-diligence",
		Title:         "Due Diligence Completa de Empresa",
		Description:   "Análise completa: dados cadastrais, compliance, judicial, setor, licitações e ESG",
		EndpointsUsed: []string{"/v1/empresas/{cnpj}/duediligence"},
		EstimatedCost: "0.075",
		Workflow:      "1. Chamar endpoint de due diligence → Retorna análise consolidada",
		AgentType:     "investment-analyst",
	},
	{
		ID:            "environmental-monitoring",
		Title:         "Monitoramento Ambiental de Município",
		Description:   "Acompanhar desmatamento, embargos do IBAMA e cobertura do solo em uma região",
		EndpointsUsed: []string{"/v1/ambiental/desmatamento", "/v1/ambiental/embargos", "/v1/ambiental/uso-solo", "/v1/ambiental/risco/{municipio}"},
		EstimatedCost: "0.022",
		Workflow:      "1. Alertas DETER recentes → 2. Embargos IBAMA ativos → 3. Cobertura do solo (MapBiomas) → 4. Score de risco do município",
		AgentType:     "environmental",
	},
	{
		ID:            "financial-market",
		Title:         "Análise de Mercado Financeiro",
		Description:   "Cotações B3, fundos CVM, câmbio e indicadores macro para decisões de investimento",
		EndpointsUsed: []string{"/v1/mercado/acoes/{ticker}", "/v1/mercado/fundos/{cnpj}", "/v1/bcb/selic", "/v1/bcb/cambio/{moeda}", "/v1/economia/panorama"},
		EstimatedCost: "0.036",
		Workflow:      "1. Cotação da ação → 2. Dados do fundo → 3. Taxa Selic atual → 4. Câmbio PTAX → 5. Panorama econômico",
		AgentType:     "financial-advisor",
	},
	{
		ID:            "employment-analysis",
		Title:         "Panorama do Mercado de Trabalho",
		Description:   "Análise de emprego formal por UF e setor usando CAGED e RAIS",
		EndpointsUsed: []string{"/v1/emprego/caged", "/v1/emprego/rais", "/v1/mercado-trabalho/{uf}/analise"},
		EstimatedCost: "0.025",
		Workflow:      "1. CAGED mensal por UF → 2. RAIS anual por UF → 3. Análise consolidada do mercado de trabalho",
		AgentType:     "labor-economist",
	},
	{
		ID:            "gazette-monitoring",
		Title:         "Monitoramento de Diários Oficiais",
		Description:   "Buscar licitações, nomeações e decisões em diários oficiais municipais",
		EndpointsUsed: []string{"/v1/diarios/busca", "/v1/diarios/municipios", "/v1/diarios/tema/{tema}"},
		EstimatedCost: "0.011",
		Workflow:      "1. Listar municípios disponíveis → 2. Buscar por palavras-chave → 3. Filtrar por tema (ex: Políticas Ambientais)",
		AgentType:     "government-monitor",
	},
	{
		ID:            "public-health",
		Title:         "Análise de Saúde Pública",
		Description:   "Dados de mortalidade, nascimentos, dengue e vacinação para pesquisa epidemiológica",
		EndpointsUsed: []string{"/v1/saude/mortalidade", "/v1/saude/nascimentos", "/v1/saude/dengue", "/v1/saude/vacinacao/{ano}"},
		EstimatedCost: "0.020",
		Workflow:      "1. Dados de mortalidade (CID-10) → 2. Nascidos vivos (SINASC) → 3. Casos de dengue → 4. Cobertura vacinal",
		AgentType:     "epidemiologist",
	},
	{
		ID:            "government-transparency",
		Title:         "Transparência Governamental",
		Description:   "Licitações, acórdãos do TCU, gastos legislativos e execução orçamentária",
		EndpointsUsed: []string{"/v1/transparencia/licitacoes", "/v1/tcu/acordaos", "/v1/legislativo/deputados/{id}/despesas", "/v1/orcamento/despesas"},
		EstimatedCost: "0.012",
		Workflow:      "1. Buscar licitações recentes → 2. Acórdãos do TCU → 3. Despesas de deputados → 4. Execução orçamentária",
		AgentType:     "civic-watchdog",
	},
}

// DiscoverHandler serves discovery case studies for AI agent onboarding.
type DiscoverHandler struct{}

// NewDiscoverHandler creates a DiscoverHandler.
func NewDiscoverHandler() *DiscoverHandler { return &DiscoverHandler{} }

// GetCases handles GET /v1/discover/cases.
// Returns hardcoded case studies showing how AI agents can use the DataBR API.
func (h *DiscoverHandler) GetCases(w http.ResponseWriter, r *http.Request) {
	respond(w, r, domain.APIResponse{
		Source:    "databr_discover",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"cases":       caseStudies,
			"total":       len(caseStudies),
			"description": "Use cases for AI agents consuming the DataBR API",
		},
	})
}
