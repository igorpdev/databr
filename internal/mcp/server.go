// Package mcp implements the DataBR MCP Server.
// Tools invoke REST handlers directly (in-process), avoiding HTTP loopback
// and the x402 payment middleware that would reject unauthenticated requests.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	mcpgosdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// HandlerDeps holds references to HTTP handlers that MCP tools invoke directly.
// On-demand handlers (always available) are non-nil; store-backed handlers may be nil
// when the database is unavailable.
type HandlerDeps struct {
	// On-demand handlers (always available — call external APIs directly)
	Empresas    http.HandlerFunc // GET /v1/empresas/{cnpj}
	Compliance  http.HandlerFunc // GET /v1/compliance/{cnpj}
	Judicial    http.HandlerFunc // GET /v1/judicial/processos/{doc}
	DOU         http.HandlerFunc // GET /v1/dou/busca
	Orcamento   http.HandlerFunc // GET /v1/orcamento/despesas
	TCU         http.HandlerFunc // GET /v1/tcu/certidao/{cnpj}
	Legislativo http.HandlerFunc // GET /v1/legislativo/deputados
	PNCP        http.HandlerFunc // GET /v1/pncp/orgaos

	// Store-backed handlers (nil when DB is unavailable)
	BCBSelic     http.HandlerFunc // GET /v1/bcb/selic
	BCBCambio    http.HandlerFunc // GET /v1/bcb/cambio/{moeda}
	EconomiaIPCA http.HandlerFunc // GET /v1/economia/ipca
	EconomiaPIB  http.HandlerFunc // GET /v1/economia/pib
	MercadoAcoes http.HandlerFunc // GET /v1/mercado/acoes/{ticker}
	Energia      http.HandlerFunc // GET /v1/energia/tarifas
	Saude        http.HandlerFunc // GET /v1/saude/medicamentos/{registro}

	// On-demand handlers (new)
	Endereco             http.HandlerFunc // GET /v1/endereco/{cep}
	Senadores            http.HandlerFunc // GET /v1/legislativo/senado/senadores
	Proposicoes          http.HandlerFunc // GET /v1/legislativo/proposicoes
	Votacoes             http.HandlerFunc // GET /v1/legislativo/votacoes
	Partidos             http.HandlerFunc // GET /v1/legislativo/partidos
	TranspServidores     http.HandlerFunc // GET /v1/transparencia/servidores
	TranspContratos      http.HandlerFunc // GET /v1/transparencia/contratos
	TranspBeneficios     http.HandlerFunc // GET /v1/transparencia/beneficios
	TranspCartoes        http.HandlerFunc // GET /v1/transparencia/cartoes
	TranspViagens        http.HandlerFunc // GET /v1/transparencia/viagens
	TranspEmendas        http.HandlerFunc // GET /v1/transparencia/emendas
	TranspObras          http.HandlerFunc // GET /v1/transparencia/obras
	TranspTransferencias http.HandlerFunc // GET /v1/transparencia/transferencias
	TranspPensionistas   http.HandlerFunc // GET /v1/transparencia/pensionistas
	TesouroRREO          http.HandlerFunc // GET /v1/tesouro/rreo
	TesouroEntes         http.HandlerFunc // GET /v1/tesouro/entes
	TesouroRGF           http.HandlerFunc // GET /v1/tesouro/rgf
	TesouroDCA           http.HandlerFunc // GET /v1/tesouro/dca
	IPEASerie            http.HandlerFunc // GET /v1/ipea/serie/{codigo}
	IPEABusca            http.HandlerFunc // GET /v1/ipea/busca
	IPEATemas            http.HandlerFunc // GET /v1/ipea/temas
	BCBIndicadores       http.HandlerFunc // GET /v1/bcb/indicadores/{serie}
	BCBIFData            http.HandlerFunc // GET /v1/bcb/ifdata
	BCBBaseMonetaria     http.HandlerFunc // GET /v1/bcb/base-monetaria
	IBGEMunicipio        http.HandlerFunc // GET /v1/ibge/municipio/{ibge}
	IBGEMunicipiosUF     http.HandlerFunc // GET /v1/ibge/municipios/{uf}
	IBGEEstados          http.HandlerFunc // GET /v1/ibge/estados
	IBGERegioes          http.HandlerFunc // GET /v1/ibge/regioes
	IBGECNAE             http.HandlerFunc // GET /v1/ibge/cnae/{codigo}
	IBGEPNAD             http.HandlerFunc // GET /v1/ibge/pnad
	IBGEINPC             http.HandlerFunc // GET /v1/ibge/inpc
	IBGEPIM              http.HandlerFunc // GET /v1/ibge/pim
	IBGEIPCA15           http.HandlerFunc // GET /v1/ibge/ipca15
	IBGEPMC              http.HandlerFunc // GET /v1/ibge/pmc
	IBGEPMS              http.HandlerFunc // GET /v1/ibge/pms
	TSEBens              http.HandlerFunc // GET /v1/eleicoes/bens
	TSEDoacoes           http.HandlerFunc // GET /v1/eleicoes/doacoes
	ANSPlanos            http.HandlerFunc // GET /v1/saude/planos
	Combustiveis         http.HandlerFunc // GET /v1/energia/combustiveis
	TCUAcordaos          http.HandlerFunc // GET /v1/tcu/acordaos
	TCUInabilitados      http.HandlerFunc // GET /v1/tcu/inabilitados
	OrcamentoFuncional   http.HandlerFunc // GET /v1/orcamento/funcional-programatica
	Diarios              http.HandlerFunc // GET /v1/diarios/busca

	// Store-backed handlers (new)
	BCBPix           http.HandlerFunc // GET /v1/bcb/pix/estatisticas
	BCBCredito       http.HandlerFunc // GET /v1/bcb/credito
	BCBTaxasCredito  http.HandlerFunc // GET /v1/bcb/taxas-credito
	BCBReservas      http.HandlerFunc // GET /v1/bcb/reservas
	EconomiaFocus    http.HandlerFunc // GET /v1/economia/focus
	IBGEPopulacao    http.HandlerFunc // GET /v1/ibge/populacao
	MercadoFundos    http.HandlerFunc // GET /v1/mercado/fundos/{cnpj}
	MercadoCotas     http.HandlerFunc // GET /v1/mercado/fundos/{cnpj}/cotas
	MercadoIbovespa  http.HandlerFunc // GET /v1/mercado/indices/ibovespa
	MercadoFatos     http.HandlerFunc // GET /v1/mercado/fatos-relevantes
	MercadoFatosById http.HandlerFunc // GET /v1/mercado/fatos-relevantes/{protocolo}
	AmbientalDesmat  http.HandlerFunc // GET /v1/ambiental/desmatamento
	AmbientalProdes  http.HandlerFunc // GET /v1/ambiental/prodes
	AmbientalUsoSolo http.HandlerFunc // GET /v1/ambiental/uso-solo
	AmbientalEmbargos http.HandlerFunc // GET /v1/ambiental/embargos
	DATASUSEstabelecimento  http.HandlerFunc // GET /v1/saude/estabelecimentos/{cnes}
	DATASUSEstabelecimentos http.HandlerFunc // GET /v1/saude/estabelecimentos
	EmpregoRAIS      http.HandlerFunc // GET /v1/emprego/rais
	EmpregoCAGED     http.HandlerFunc // GET /v1/emprego/caged
	TranspAeronave   http.HandlerFunc // GET /v1/transporte/aeronaves/{prefixo}
	TranspAeronaves  http.HandlerFunc // GET /v1/transporte/aeronaves
	Transportador    http.HandlerFunc // GET /v1/transporte/transportadores/{rntrc}
	Transportadores  http.HandlerFunc // GET /v1/transporte/transportadores
	TranspAcidentes  http.HandlerFunc // GET /v1/transporte/acidentes
	ComexExportacoes http.HandlerFunc // GET /v1/comercio/exportacoes
	ComexImportacoes http.HandlerFunc // GET /v1/comercio/importacoes
	CandidatosTSE    http.HandlerFunc // GET /v1/eleicoes/candidatos
	TesouroTitulos   http.HandlerFunc // GET /v1/tesouro/titulos
	CensoEscolar     http.HandlerFunc // GET /v1/educacao/censo-escolar
	EnergiaGeracao   http.HandlerFunc // GET /v1/energia/geracao
	EnergiaCarga     http.HandlerFunc // GET /v1/energia/carga
	FundoAnalise     http.HandlerFunc // GET /v1/mercado/fundos/{cnpj}/analise

	// Phase 4-5 handlers
	JudicialProcesso    http.HandlerFunc // GET /v1/judicial/processo/{numero}
	JudicialSTF         http.HandlerFunc // GET /v1/judicial/stf
	JudicialSTJ         http.HandlerFunc // GET /v1/judicial/stj
	DATASUSMortalidade  http.HandlerFunc // GET /v1/saude/mortalidade
	DATASUSNascimentos  http.HandlerFunc // GET /v1/saude/nascimentos
	DATASUSHospitais    http.HandlerFunc // GET /v1/saude/hospitais
	DATASUSDengue       http.HandlerFunc // GET /v1/saude/dengue
	DATASUSVacinacao    http.HandlerFunc // GET /v1/saude/vacinacao/{ano}
	DiariosMunicipios   http.HandlerFunc // GET /v1/diarios/municipios
	DiariosTemas        http.HandlerFunc // GET /v1/diarios/temas
	DiariosTema         http.HandlerFunc // GET /v1/diarios/tema/{tema}
	DiscoverCases       http.HandlerFunc // GET /v1/discover/cases

	// Premium/Composite handlers (new)
	DueDiligence        http.HandlerFunc // GET /v1/empresas/{cnpj}/duediligence
	PerfilCompleto      http.HandlerFunc // GET /v1/empresas/{cnpj}/perfil-completo
	CreditoScore        http.HandlerFunc // GET /v1/credito/score/{cnpj}
	Panorama            http.HandlerFunc // GET /v1/economia/panorama
	SetorAnalise        http.HandlerFunc // GET /v1/empresas/{cnpj}/setor
	RegulacaoSetor      http.HandlerFunc // GET /v1/setor/{cnae}/regulacao
	Competicao          http.HandlerFunc // GET /v1/mercado/{cnae}/competicao
	MercadoTrabalho     http.HandlerFunc // GET /v1/mercado-trabalho/{uf}/analise
	ESG                 http.HandlerFunc // GET /v1/ambiental/empresa/{cnpj}/esg
	RiscoAmbiental      http.HandlerFunc // GET /v1/ambiental/risco/{municipio}
	LitigioRisco        http.HandlerFunc // GET /v1/litigio/{cnpj}/risco
	RedeInfluencia      http.HandlerFunc // GET /v1/rede/{cnpj}/influencia
	MunicipioPerfil     http.HandlerFunc // GET /v1/municipios/{codigo}/perfil
	ComplianceEleitoral http.HandlerFunc // GET /v1/eleicoes/compliance/{cpf_cnpj}

	// Tributario handlers
	TributarioNCM  http.HandlerFunc // GET /v1/tributario/ncm/{codigo}
	TributarioICMS http.HandlerFunc // GET /v1/tributario/icms/{uf} or /v1/tributario/icms

	// Transparência Federal (new endpoints)
	TranspPGFN      http.HandlerFunc // GET /v1/transparencia/pgfn
	TranspPEP       http.HandlerFunc // GET /v1/transparencia/pep
	TranspLeniencias http.HandlerFunc // GET /v1/transparencia/leniencias
	TranspRenuncias  http.HandlerFunc // GET /v1/transparencia/renuncias

	// BNDES
	BNDESOperacoes http.HandlerFunc // GET /v1/bndes/{cnpj}/operacoes

	// TSE Filiados
	TSEFiliados http.HandlerFunc // GET /v1/eleicoes/filiados
}

// ToolPrices maps each MCP tool name to its USDC price string.
// Used by NewPerToolMiddleware to apply per-call x402 payment gates.
var ToolPrices = map[string]string{
	// Existing tools
	"consultar_empresa":          "0.003",
	"verificar_compliance":       "0.010",
	"cotacao_cambio":             "0.003",
	"indicadores_macro":          "0.003",
	"buscar_processos_judiciais": "0.015",
	"buscar_diario_oficial":      "0.007",
	"consultar_orcamento":        "0.003",
	"consultar_tcu_certidao":     "0.003",
	"cotacao_acoes":              "0.005",
	"consultar_deputados":        "0.003",
	"buscar_licitacao":           "0.003",
	"consultar_tarifas_energia":  "0.003",
	"consultar_medicamento":      "0.003",
	// BCB
	"consultar_selic":                   "0.003",
	"consultar_pix_estatisticas":        "0.003",
	"consultar_credito_bcb":             "0.003",
	"consultar_reservas_internacionais": "0.003",
	"consultar_focus":                   "0.003",
	"consultar_indicador_bcb":           "0.003",
	// IBGE
	"consultar_populacao":        "0.003",
	"consultar_municipio":        "0.003",
	"consultar_geografia_brasil": "0.003",
	"consultar_cnae":             "0.003",
	"consultar_indicadores_ibge": "0.003",
	// Transparência Federal
	"consultar_servidores_federais": "0.003",
	"consultar_contratos_federais":  "0.003",
	"consultar_beneficios_sociais":  "0.003",
	"consultar_transparencia":       "0.003",
	// Tesouro
	"consultar_titulos_tesouro": "0.003",
	"consultar_contas_publicas": "0.003",
	// Legislativo
	"consultar_senadores":          "0.003",
	"consultar_proposicoes":        "0.003",
	"consultar_votacoes":           "0.003",
	"consultar_partidos":           "0.003",
	"consultar_diarios_municipais": "0.007",
	"consultar_orcamento_funcional": "0.003",
	// Eleitoral
	"consultar_candidatos":              "0.003",
	"consultar_financiamento_eleitoral": "0.007",
	// CVM / Mercado
	"consultar_fundo_investimento": "0.010",
	"consultar_ibovespa":           "0.005",
	"consultar_fatos_relevantes":   "0.005",
	// TCU expanded
	"consultar_tcu_acordaos":     "0.003",
	"consultar_tcu_inabilitados": "0.003",
	// Ambiental
	"consultar_desmatamento":        "0.005",
	"consultar_embargos_ambientais": "0.005",
	"consultar_uso_solo":            "0.005",
	// Emprego
	"consultar_emprego": "0.005",
	// Transporte
	"consultar_aeronave":           "0.005",
	"consultar_transportador":      "0.005",
	"consultar_acidentes_transito": "0.005",
	// Comércio Exterior
	"consultar_comercio_exterior": "0.005",
	// Outros
	"consultar_combustiveis":  "0.003",
	"consultar_planos_saude":              "0.003",
	"consultar_estabelecimento_saude":     "0.003",
	"consultar_ipea":          "0.003",
	"consultar_endereco":      "0.003",
	"consultar_censo_escolar": "0.005",
	// Premium / Composite
	"due_diligence_empresa":    "0.075",
	"perfil_completo_empresa":  "0.020",
	"credito_score":            "0.010",
	"panorama_economico":       "0.015",
	"analise_setor":            "0.020",
	"analise_competicao":       "0.030",
	"analise_mercado_trabalho": "0.015",
	"analise_esg":              "0.030",
	"risco_ambiental":          "0.007",
	"analise_litigio":          "0.030",
	"rede_influencia":          "0.050",
	"consultar_jurisprudencia": "0.010",
	"perfil_municipio":         "0.007",
	// Phase 4-5 tools
	"consultar_processo_judicial": "0.010",
	"consultar_mortalidade":       "0.005",
	"consultar_nascimentos":       "0.005",
	"consultar_hospitais":         "0.005",
	"consultar_dengue":            "0.005",
	"consultar_vacinacao":         "0.005",
	"listar_municipios_diarios":   "0.003",
	"listar_temas_diarios":        "0.003",
	"buscar_diarios_por_tema":     "0.005",
	"descobrir_casos_uso":         "0",
	// Tributário
	"consultar_tributos_ncm": "0.003",
	"consultar_icms":         "0.003",
	// Transparência Federal (new)
	"consultar_pgfn":              "0.003",
	"consultar_pep":               "0.003",
	"consultar_leniencias":        "0.003",
	"consultar_renuncias_fiscais": "0.003",
	// BNDES
	"consultar_bndes": "0.005",
	// TSE Filiados
	"consultar_filiados_tse": "0.003",
}

// Server wraps the mcp-go server with DataBR tool registrations.
type Server struct {
	mcpServer *server.MCPServer
	deps      *HandlerDeps
	tools     []string
}

// NewServer creates a DataBR MCP Server that invokes handlers directly (in-process).
func NewServer(deps *HandlerDeps) *Server {
	if deps == nil {
		deps = &HandlerDeps{}
	}

	s := &Server{
		mcpServer: server.NewMCPServer(
			"DataBR",
			"1.0.0",
			server.WithToolCapabilities(true),
		),
		deps: deps,
	}

	s.registerTools()
	return s
}

// Tools returns the names of all registered MCP tools.
func (s *Server) Tools() []string {
	return s.tools
}

// MCPServer returns the underlying mcp-go server (for mounting in HTTP handler).
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// maxResponseBytes limits handler response body size to prevent OOM (10 MB).
const maxResponseBytes = 10 << 20

// invokeHandler calls a handler function directly, injecting Chi URL params and query params.
// Returns the response body as a tool result, or an error if the handler is nil or returns >= 400.
func invokeHandler(ctx context.Context, handler http.HandlerFunc, path string, chiParams map[string]string, query string) (*mcpgosdk.CallToolResult, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler not available (database may be disconnected)")
	}

	url := path
	if query != "" {
		url = path + "?" + query
	}

	req := httptest.NewRequest(http.MethodGet, url, nil)
	req = req.WithContext(ctx)

	// Inject Chi URL params so handlers can use chi.URLParam(r, "key").
	if len(chiParams) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range chiParams {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.Bytes()
	if len(body) > maxResponseBytes {
		body = body[:maxResponseBytes]
	}

	if rec.Code >= 400 {
		return nil, fmt.Errorf("handler returned %d: %s", rec.Code, string(body))
	}

	return mcpgosdk.NewToolResultText(string(body)), nil
}

func (s *Server) registerTools() {
	s.addTool("consultar_empresa",
		"Consulta dados completos de empresa brasileira por CNPJ. Retorna razão social, situação cadastral, endereço, atividade econômica (CNAE) e sócios.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj",
				mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa, com ou sem formatação"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.Empresas, "/v1/empresas/"+cnpj, map[string]string{"cnpj": cnpj}, "")
		},
	)

	s.addTool("verificar_compliance",
		"Verifica pendências de compliance de empresa no CEIS (empresa sancionada) e CNEP (empresa punida) do Portal da Transparência / CGU.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj",
				mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.Compliance, "/v1/compliance/"+cnpj, map[string]string{"cnpj": cnpj}, "")
		},
	)

	s.addTool("cotacao_cambio",
		"Retorna a taxa de câmbio PTAX do Banco Central do Brasil para a moeda solicitada (compra e venda).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("moeda",
				mcpgosdk.Required(),
				mcpgosdk.Description("Código da moeda (ex: USD, EUR, GBP, JPY, ARS)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			moeda := req.GetString("moeda", "USD")
			return invokeHandler(ctx, s.deps.BCBCambio, "/v1/bcb/cambio/"+moeda, map[string]string{"moeda": moeda}, "")
		},
	)

	s.addTool("indicadores_macro",
		"Retorna indicadores macroeconômicos do Brasil: IPCA (inflação), Selic (juros), PIB e câmbio USD.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			result := map[string]any{}

			if r, err := invokeHandler(ctx, s.deps.BCBSelic, "/v1/bcb/selic", nil, ""); err == nil {
				result["selic"] = extractJSON(r)
			}
			if r, err := invokeHandler(ctx, s.deps.EconomiaIPCA, "/v1/economia/ipca", nil, ""); err == nil {
				result["ipca"] = extractJSON(r)
			}
			if r, err := invokeHandler(ctx, s.deps.EconomiaPIB, "/v1/economia/pib", nil, ""); err == nil {
				result["pib"] = extractJSON(r)
			}
			if r, err := invokeHandler(ctx, s.deps.BCBCambio, "/v1/bcb/cambio/USD", map[string]string{"moeda": "USD"}, ""); err == nil {
				result["cambio"] = extractJSON(r)
			}

			if len(result) == 0 {
				return nil, fmt.Errorf("no macro indicators available (database may be disconnected)")
			}

			b, _ := json.Marshal(result)
			return mcpgosdk.NewToolResultText(string(b)), nil
		},
	)

	s.addTool("buscar_processos_judiciais",
		"Busca processos judiciais por CPF ou CNPJ no DataJud CNJ (todos os tribunais).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("documento",
				mcpgosdk.Required(),
				mcpgosdk.Description("CPF ou CNPJ do interessado no processo"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			doc := req.GetString("documento", "")
			return invokeHandler(ctx, s.deps.Judicial, "/v1/judicial/processos/"+doc, map[string]string{"doc": doc}, "")
		},
	)

	s.addTool("buscar_diario_oficial",
		"Busca publicações no Diário Oficial Municipal via Querido Diário (OK.org.br).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("query",
				mcpgosdk.Required(),
				mcpgosdk.Description("Termo de busca (ex: nome de empresa, licitação, contrato)"),
			),
			mcpgosdk.WithString("uf",
				mcpgosdk.Description("Sigla do estado (ex: SP, RJ). Opcional."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			q := req.GetString("query", "")
			uf := req.GetString("uf", "")
			query := "q=" + q
			if uf != "" {
				query += "&uf=" + uf
			}
			return invokeHandler(ctx, s.deps.DOU, "/v1/dou/busca", nil, query)
		},
	)

	s.addTool("consultar_orcamento",
		"Consulta despesas do orçamento federal por órgão e ano (dados SIAFI via Portal da Transparência).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("ano", mcpgosdk.Required(), mcpgosdk.Description("Ano do orçamento (ex: 2025)")),
			mcpgosdk.WithString("orgao", mcpgosdk.Description("Código SIAFI do órgão (ex: 26000 para MEC). Opcional.")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			ano := req.GetString("ano", "")
			orgao := req.GetString("orgao", "")
			query := "ano=" + ano
			if orgao != "" {
				query += "&orgao=" + orgao
			}
			return invokeHandler(ctx, s.deps.Orcamento, "/v1/orcamento/despesas", nil, query)
		},
	)

	s.addTool("consultar_tcu_certidao",
		"Verifica certidão de regularidade de empresa no TCU (Tribunal de Contas da União).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(), mcpgosdk.Description("CNPJ da empresa")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.TCU, "/v1/tcu/certidao/"+cnpj, map[string]string{"cnpj": cnpj}, "")
		},
	)

	s.addTool("cotacao_acoes",
		"Retorna cotação de ação na B3 (Bolsa de Valores do Brasil) pelo ticker.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("ticker", mcpgosdk.Required(), mcpgosdk.Description("Ticker da ação (ex: PETR4, VALE3, ITUB4)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			ticker := req.GetString("ticker", "")
			return invokeHandler(ctx, s.deps.MercadoAcoes, "/v1/mercado/acoes/"+ticker, map[string]string{"ticker": ticker}, "")
		},
	)

	s.addTool("consultar_deputados",
		"Busca deputados federais na Câmara dos Deputados. Filtre por UF e/ou partido.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("uf", mcpgosdk.Description("Sigla do estado (ex: SP, RJ). Opcional.")),
			mcpgosdk.WithString("partido", mcpgosdk.Description("Sigla do partido (ex: PT, PL). Opcional.")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			uf := req.GetString("uf", "")
			partido := req.GetString("partido", "")
			var query string
			if uf != "" {
				query += "uf=" + uf + "&"
			}
			if partido != "" {
				query += "partido=" + partido
			}
			return invokeHandler(ctx, s.deps.Legislativo, "/v1/legislativo/deputados", nil, query)
		},
	)

	s.addTool("buscar_licitacao",
		"Busca licitações e contratações públicas no PNCP (Portal Nacional de Contratações Públicas).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(), mcpgosdk.Description("CNPJ do órgão contratante")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.PNCP, "/v1/pncp/orgaos", nil, "cnpj="+cnpj)
		},
	)

	s.addTool("consultar_tarifas_energia",
		"Retorna tarifas de energia elétrica da ANEEL por distribuidora.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("distribuidora", mcpgosdk.Description("Nome da distribuidora (ex: ENEL, CEMIG). Opcional.")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			dist := req.GetString("distribuidora", "")
			var query string
			if dist != "" {
				query = "distribuidora=" + dist
			}
			return invokeHandler(ctx, s.deps.Energia, "/v1/energia/tarifas", nil, query)
		},
	)

	s.addTool("consultar_medicamento",
		"Busca medicamento registrado na ANVISA pelo número de registro.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("registro", mcpgosdk.Required(), mcpgosdk.Description("Número de registro ANVISA do medicamento")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			registro := req.GetString("registro", "")
			return invokeHandler(ctx, s.deps.Saude, "/v1/saude/medicamentos/"+registro, map[string]string{"registro": registro}, "")
		},
	)

	// === BCB expanded ===
	s.addTool("consultar_selic",
		"Retorna a taxa Selic atual e histórico recente do Banco Central do Brasil.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.BCBSelic, "/v1/bcb/selic", nil, "")
		},
	)

	s.addTool("consultar_pix_estatisticas",
		"Retorna estatísticas do sistema de pagamentos PIX (volume, quantidade de transações).",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.BCBPix, "/v1/bcb/pix/estatisticas", nil, "")
		},
	)

	s.addTool("consultar_credito_bcb",
		"Retorna dados de crédito do BCB: volume de crédito ou taxas de juros praticadas.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tipo",
				mcpgosdk.Description("Tipo de consulta: 'volume' para volume de crédito, 'taxas' para taxas de juros. Padrão: volume."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tipo := req.GetString("tipo", "volume")
			if tipo == "taxas" {
				return invokeHandler(ctx, s.deps.BCBTaxasCredito, "/v1/bcb/taxas-credito", nil, "")
			}
			return invokeHandler(ctx, s.deps.BCBCredito, "/v1/bcb/credito", nil, "")
		},
	)

	s.addTool("consultar_reservas_internacionais",
		"Retorna dados de reservas internacionais do Brasil (BCB).",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.BCBReservas, "/v1/bcb/reservas", nil, "")
		},
	)

	s.addTool("consultar_focus",
		"Retorna relatório Focus do BCB com expectativas de mercado (IPCA, Selic, PIB, câmbio).",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.EconomiaFocus, "/v1/economia/focus", nil, "")
		},
	)

	s.addTool("consultar_indicador_bcb",
		"Consulta indicadores do BCB por código SGS, ou serviços OLINDA (IFDATA, base monetária).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("serie",
				mcpgosdk.Description("Código da série SGS (ex: 432 para IPCA). Quando omitido, use 'servico'."),
			),
			mcpgosdk.WithString("servico",
				mcpgosdk.Description("Serviço OLINDA: 'ifdata', 'base-monetaria'. Usado quando 'serie' não é informado."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			serie := req.GetString("serie", "")
			servico := req.GetString("servico", "")
			if serie != "" {
				return invokeHandler(ctx, s.deps.BCBIndicadores, "/v1/bcb/indicadores/"+serie, map[string]string{"serie": serie}, "")
			}
			switch servico {
			case "ifdata":
				return invokeHandler(ctx, s.deps.BCBIFData, "/v1/bcb/ifdata", nil, "")
			case "base-monetaria":
				return invokeHandler(ctx, s.deps.BCBBaseMonetaria, "/v1/bcb/base-monetaria", nil, "")
			default:
				return nil, fmt.Errorf("informe 'serie' (código SGS) ou 'servico' (ifdata, base-monetaria)")
			}
		},
	)

	// === IBGE ===
	s.addTool("consultar_populacao",
		"Retorna estimativas populacionais do Brasil por estado e município (IBGE).",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.IBGEPopulacao, "/v1/ibge/populacao", nil, "")
		},
	)

	s.addTool("consultar_municipio",
		"Consulta dados de município brasileiro por código IBGE ou lista municípios de um estado (UF).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("codigo",
				mcpgosdk.Description("Código IBGE do município (7 dígitos). Alternativo a 'uf'."),
			),
			mcpgosdk.WithString("uf",
				mcpgosdk.Description("Sigla do estado (ex: SP, RJ) para listar municípios. Alternativo a 'codigo'."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			codigo := req.GetString("codigo", "")
			uf := req.GetString("uf", "")
			if codigo != "" {
				return invokeHandler(ctx, s.deps.IBGEMunicipio, "/v1/ibge/municipio/"+codigo, map[string]string{"ibge": codigo}, "")
			}
			if uf != "" {
				return invokeHandler(ctx, s.deps.IBGEMunicipiosUF, "/v1/ibge/municipios/"+uf, map[string]string{"uf": uf}, "")
			}
			return nil, fmt.Errorf("informe 'codigo' (IBGE) ou 'uf' (sigla do estado)")
		},
	)

	s.addTool("consultar_geografia_brasil",
		"Retorna estados ou regiões do Brasil (IBGE).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tipo",
				mcpgosdk.Description("Tipo: 'estados' ou 'regioes'. Padrão: estados."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tipo := req.GetString("tipo", "estados")
			if tipo == "regioes" {
				return invokeHandler(ctx, s.deps.IBGERegioes, "/v1/ibge/regioes", nil, "")
			}
			return invokeHandler(ctx, s.deps.IBGEEstados, "/v1/ibge/estados", nil, "")
		},
	)

	s.addTool("consultar_cnae",
		"Consulta atividade econômica (CNAE) por código. Retorna descrição, divisão e seção.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("codigo", mcpgosdk.Required(),
				mcpgosdk.Description("Código CNAE (ex: 6201-5 para desenvolvimento de software)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			codigo := req.GetString("codigo", "")
			return invokeHandler(ctx, s.deps.IBGECNAE, "/v1/ibge/cnae/"+codigo, map[string]string{"codigo": codigo}, "")
		},
	)

	s.addTool("consultar_indicadores_ibge",
		"Retorna indicadores econômicos do IBGE: PNAD, INPC, PIM, IPCA-15, PMC, PMS.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("indicador", mcpgosdk.Required(),
				mcpgosdk.Description("Nome: 'pnad' (emprego), 'inpc' (inflação), 'pim' (indústria), 'ipca15' (prévias), 'pmc' (comércio), 'pms' (serviços)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			indicador := req.GetString("indicador", "pnad")
			handlerMap := map[string]http.HandlerFunc{
				"pnad":   s.deps.IBGEPNAD,
				"inpc":   s.deps.IBGEINPC,
				"pim":    s.deps.IBGEPIM,
				"ipca15": s.deps.IBGEIPCA15,
				"pmc":    s.deps.IBGEPMC,
				"pms":    s.deps.IBGEPMS,
			}
			h, ok := handlerMap[indicador]
			if !ok {
				return nil, fmt.Errorf("indicador '%s' não reconhecido; use: pnad, inpc, pim, ipca15, pmc, pms", indicador)
			}
			return invokeHandler(ctx, h, "/v1/ibge/"+indicador, nil, "")
		},
	)

	// === Transparência Federal ===
	s.addTool("consultar_servidores_federais",
		"Consulta servidores públicos federais por CPF ou nome (Portal da Transparência / CGU).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cpf", mcpgosdk.Description("CPF do servidor")),
			mcpgosdk.WithString("nome", mcpgosdk.Description("Nome do servidor")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cpf := req.GetString("cpf", "")
			nome := req.GetString("nome", "")
			query := ""
			if cpf != "" {
				query = "cpf=" + cpf
			} else if nome != "" {
				query = "nome=" + nome
			}
			return invokeHandler(ctx, s.deps.TranspServidores, "/v1/transparencia/servidores", nil, query)
		},
	)

	s.addTool("consultar_contratos_federais",
		"Consulta contratos federais no Portal da Transparência (CGU).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Description("CNPJ do contratado")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			query := ""
			if cnpj != "" {
				query = "cnpj=" + cnpj
			}
			return invokeHandler(ctx, s.deps.TranspContratos, "/v1/transparencia/contratos", nil, query)
		},
	)

	s.addTool("consultar_beneficios_sociais",
		"Consulta beneficiários de programas sociais federais (Bolsa Família) por município.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("municipio", mcpgosdk.Description("Código IBGE do município")),
			mcpgosdk.WithString("uf", mcpgosdk.Description("Sigla do estado (ex: SP)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			municipio := req.GetString("municipio", "")
			uf := req.GetString("uf", "")
			query := ""
			if municipio != "" {
				query = "municipio=" + municipio
			}
			if uf != "" {
				if query != "" {
					query += "&"
				}
				query += "uf=" + uf
			}
			return invokeHandler(ctx, s.deps.TranspBeneficios, "/v1/transparencia/beneficios", nil, query)
		},
	)

	s.addTool("consultar_transparencia",
		"Consulta dados de transparência federal: cartões corporativos, viagens, emendas, obras, transferências, pensionistas.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tipo", mcpgosdk.Required(),
				mcpgosdk.Description("Tipo: 'cartoes', 'viagens', 'emendas', 'obras', 'transferencias', 'pensionistas'"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tipo := req.GetString("tipo", "")
			handlerMap := map[string]struct {
				h    http.HandlerFunc
				path string
			}{
				"cartoes":        {s.deps.TranspCartoes, "/v1/transparencia/cartoes"},
				"viagens":        {s.deps.TranspViagens, "/v1/transparencia/viagens"},
				"emendas":        {s.deps.TranspEmendas, "/v1/transparencia/emendas"},
				"obras":          {s.deps.TranspObras, "/v1/transparencia/obras"},
				"transferencias": {s.deps.TranspTransferencias, "/v1/transparencia/transferencias"},
				"pensionistas":   {s.deps.TranspPensionistas, "/v1/transparencia/pensionistas"},
			}
			entry, ok := handlerMap[tipo]
			if !ok {
				return nil, fmt.Errorf("tipo '%s' não reconhecido; use: cartoes, viagens, emendas, obras, transferencias, pensionistas", tipo)
			}
			return invokeHandler(ctx, entry.h, entry.path, nil, "")
		},
	)

	// === Tesouro ===
	s.addTool("consultar_titulos_tesouro",
		"Retorna dados do Tesouro Direto: títulos disponíveis, taxas e preços.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.TesouroTitulos, "/v1/tesouro/titulos", nil, "")
		},
	)

	s.addTool("consultar_contas_publicas",
		"Consulta dados de finanças públicas do Tesouro Nacional: RREO, RGF, DCA, entes federados.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tipo", mcpgosdk.Required(),
				mcpgosdk.Description("Tipo: 'rreo' (execução), 'rgf' (gestão fiscal), 'dca' (disponibilidades), 'entes' (entes federados)"),
			),
			mcpgosdk.WithString("ano", mcpgosdk.Description("Ano de referência (ex: 2025)")),
			mcpgosdk.WithString("ente", mcpgosdk.Description("Código do ente federado")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tipo := req.GetString("tipo", "")
			ano := req.GetString("ano", "")
			ente := req.GetString("ente", "")
			query := ""
			if ano != "" {
				query = "an_exercicio=" + ano
			}
			if ente != "" {
				if query != "" {
					query += "&"
				}
				query += "id_ente=" + ente
			}
			handlerMap := map[string]struct {
				h    http.HandlerFunc
				path string
			}{
				"rreo":  {s.deps.TesouroRREO, "/v1/tesouro/rreo"},
				"rgf":   {s.deps.TesouroRGF, "/v1/tesouro/rgf"},
				"dca":   {s.deps.TesouroDCA, "/v1/tesouro/dca"},
				"entes": {s.deps.TesouroEntes, "/v1/tesouro/entes"},
			}
			entry, ok := handlerMap[tipo]
			if !ok {
				return nil, fmt.Errorf("tipo '%s' não reconhecido; use: rreo, rgf, dca, entes", tipo)
			}
			return invokeHandler(ctx, entry.h, entry.path, nil, query)
		},
	)

	// === Legislativo expanded ===
	s.addTool("consultar_senadores",
		"Busca senadores federais no Senado Federal. Filtre por UF e/ou partido.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("uf", mcpgosdk.Description("Sigla do estado (ex: SP, RJ)")),
			mcpgosdk.WithString("partido", mcpgosdk.Description("Sigla do partido (ex: PT, PL)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			uf := req.GetString("uf", "")
			partido := req.GetString("partido", "")
			query := ""
			if uf != "" {
				query += "uf=" + uf + "&"
			}
			if partido != "" {
				query += "partido=" + partido
			}
			return invokeHandler(ctx, s.deps.Senadores, "/v1/legislativo/senado/senadores", nil, query)
		},
	)

	s.addTool("consultar_proposicoes",
		"Busca proposições legislativas (projetos de lei) na Câmara dos Deputados.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tema", mcpgosdk.Description("Tema da proposição")),
			mcpgosdk.WithString("ano", mcpgosdk.Description("Ano de apresentação (ex: 2025)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tema := req.GetString("tema", "")
			ano := req.GetString("ano", "")
			query := ""
			if tema != "" {
				query += "tema=" + tema + "&"
			}
			if ano != "" {
				query += "ano=" + ano
			}
			return invokeHandler(ctx, s.deps.Proposicoes, "/v1/legislativo/proposicoes", nil, query)
		},
	)

	s.addTool("consultar_votacoes",
		"Retorna votações recentes na Câmara dos Deputados.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.Votacoes, "/v1/legislativo/votacoes", nil, "")
		},
	)

	s.addTool("consultar_partidos",
		"Lista partidos políticos, frentes parlamentares ou blocos partidários na Câmara.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tipo",
				mcpgosdk.Description("Tipo: 'partidos', 'frentes' (parlamentares), 'blocos' (partidários). Padrão: partidos."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tipo := req.GetString("tipo", "partidos")
			switch tipo {
			case "frentes":
				return invokeHandler(ctx, s.deps.Partidos, "/v1/legislativo/frentes", nil, "")
			case "blocos":
				return invokeHandler(ctx, s.deps.Partidos, "/v1/legislativo/blocos", nil, "")
			default:
				return invokeHandler(ctx, s.deps.Partidos, "/v1/legislativo/partidos", nil, "")
			}
		},
	)

	s.addTool("consultar_diarios_municipais",
		"Busca publicações em diários oficiais municipais via Querido Diário (OK.org.br).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("query", mcpgosdk.Required(),
				mcpgosdk.Description("Termo de busca (ex: nome de empresa, licitação)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			q := req.GetString("query", "")
			return invokeHandler(ctx, s.deps.Diarios, "/v1/diarios/busca", nil, "q="+q)
		},
	)

	s.addTool("consultar_orcamento_funcional",
		"Consulta orçamento federal por classificação funcional-programática.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("ano", mcpgosdk.Description("Ano do orçamento (ex: 2025)")),
			mcpgosdk.WithString("funcao", mcpgosdk.Description("Código da função (ex: 12 para educação)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			ano := req.GetString("ano", "")
			funcao := req.GetString("funcao", "")
			query := ""
			if ano != "" {
				query = "ano=" + ano
			}
			if funcao != "" {
				if query != "" {
					query += "&"
				}
				query += "funcao=" + funcao
			}
			return invokeHandler(ctx, s.deps.OrcamentoFuncional, "/v1/orcamento/funcional-programatica", nil, query)
		},
	)

	// === Eleitoral ===
	s.addTool("consultar_candidatos",
		"Busca candidatos a cargos eletivos no TSE (Tribunal Superior Eleitoral).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("nome", mcpgosdk.Description("Nome do candidato")),
			mcpgosdk.WithString("uf", mcpgosdk.Description("Sigla do estado (ex: SP)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.CandidatosTSE, "/v1/eleicoes/candidatos", nil, "")
		},
	)

	s.addTool("consultar_financiamento_eleitoral",
		"Consulta bens declarados ou doações de campanha de candidatos (TSE).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tipo", mcpgosdk.Required(),
				mcpgosdk.Description("Tipo: 'bens' (patrimônio declarado) ou 'doacoes' (doações recebidas)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tipo := req.GetString("tipo", "bens")
			if tipo == "doacoes" {
				return invokeHandler(ctx, s.deps.TSEDoacoes, "/v1/eleicoes/doacoes", nil, "")
			}
			return invokeHandler(ctx, s.deps.TSEBens, "/v1/eleicoes/bens", nil, "")
		},
	)

	// === CVM / Mercado ===
	s.addTool("consultar_fundo_investimento",
		"Consulta fundos de investimento pela CVM: dados cadastrais, cotas diárias ou análise.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ do fundo de investimento"),
			),
			mcpgosdk.WithString("detalhe",
				mcpgosdk.Description("Nível: 'cadastro', 'cotas' (histórico), 'analise'. Padrão: cadastro."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			detalhe := req.GetString("detalhe", "cadastro")
			switch detalhe {
			case "cotas":
				return invokeHandler(ctx, s.deps.MercadoCotas, "/v1/mercado/fundos/"+cnpj+"/cotas", map[string]string{"cnpj": cnpj}, "")
			case "analise":
				return invokeHandler(ctx, s.deps.FundoAnalise, "/v1/mercado/fundos/"+cnpj+"/analise", map[string]string{"cnpj": cnpj}, "")
			default:
				return invokeHandler(ctx, s.deps.MercadoFundos, "/v1/mercado/fundos/"+cnpj, map[string]string{"cnpj": cnpj}, "")
			}
		},
	)

	s.addTool("consultar_ibovespa",
		"Retorna composição e cotação do índice IBOVESPA (B3).",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.MercadoIbovespa, "/v1/mercado/indices/ibovespa", nil, "")
		},
	)

	s.addTool("consultar_fatos_relevantes",
		"Busca fatos relevantes publicados por empresas na CVM. Pode buscar por protocolo.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("protocolo",
				mcpgosdk.Description("Número do protocolo CVM (opcional). Quando omitido, retorna últimos fatos."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			protocolo := req.GetString("protocolo", "")
			if protocolo != "" {
				return invokeHandler(ctx, s.deps.MercadoFatosById, "/v1/mercado/fatos-relevantes/"+protocolo, map[string]string{"protocolo": protocolo}, "")
			}
			return invokeHandler(ctx, s.deps.MercadoFatos, "/v1/mercado/fatos-relevantes", nil, "")
		},
	)

	// === TCU expanded ===
	s.addTool("consultar_tcu_acordaos",
		"Busca acórdãos (decisões) do TCU (Tribunal de Contas da União).",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.TCUAcordaos, "/v1/tcu/acordaos", nil, "")
		},
	)

	s.addTool("consultar_tcu_inabilitados",
		"Lista pessoas inabilitadas pelo TCU para exercer cargo público.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.TCUInabilitados, "/v1/tcu/inabilitados", nil, "")
		},
	)

	// === Ambiental ===
	s.addTool("consultar_desmatamento",
		"Retorna dados de desmatamento na Amazônia: alertas DETER (tempo real) ou PRODES (anual).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tipo",
				mcpgosdk.Description("Tipo: 'deter' (alertas tempo real) ou 'prodes' (consolidado anual). Padrão: deter."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tipo := req.GetString("tipo", "deter")
			if tipo == "prodes" {
				return invokeHandler(ctx, s.deps.AmbientalProdes, "/v1/ambiental/prodes", nil, "")
			}
			return invokeHandler(ctx, s.deps.AmbientalDesmat, "/v1/ambiental/desmatamento", nil, "")
		},
	)

	s.addTool("consultar_embargos_ambientais",
		"Lista embargos do IBAMA: áreas interditadas por infrações ambientais.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.AmbientalEmbargos, "/v1/ambiental/embargos", nil, "")
		},
	)

	s.addTool("consultar_uso_solo",
		"Retorna dados de classificação de uso do solo (MapBiomas).",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.AmbientalUsoSolo, "/v1/ambiental/uso-solo", nil, "")
		},
	)

	// === Emprego ===
	s.addTool("consultar_emprego",
		"Retorna dados de emprego formal no Brasil: RAIS (anual) ou CAGED (mensal).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tipo",
				mcpgosdk.Description("Tipo: 'rais' (anual) ou 'caged' (mensal). Padrão: caged."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tipo := req.GetString("tipo", "caged")
			if tipo == "rais" {
				return invokeHandler(ctx, s.deps.EmpregoRAIS, "/v1/emprego/rais", nil, "")
			}
			return invokeHandler(ctx, s.deps.EmpregoCAGED, "/v1/emprego/caged", nil, "")
		},
	)

	// === Transporte ===
	s.addTool("consultar_aeronave",
		"Consulta aeronaves no RAB (Registro Aeronáutico Brasileiro) da ANAC.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("prefixo",
				mcpgosdk.Description("Prefixo da aeronave (ex: PT-ABC). Quando omitido, lista aeronaves."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			prefixo := req.GetString("prefixo", "")
			if prefixo != "" {
				return invokeHandler(ctx, s.deps.TranspAeronave, "/v1/transporte/aeronaves/"+prefixo, map[string]string{"prefixo": prefixo}, "")
			}
			return invokeHandler(ctx, s.deps.TranspAeronaves, "/v1/transporte/aeronaves", nil, "")
		},
	)

	s.addTool("consultar_transportador",
		"Consulta transportadores rodoviários no RNTRC (ANTT).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("rntrc",
				mcpgosdk.Description("Número RNTRC. Quando omitido, lista transportadores."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			rntrc := req.GetString("rntrc", "")
			if rntrc != "" {
				return invokeHandler(ctx, s.deps.Transportador, "/v1/transporte/transportadores/"+rntrc, map[string]string{"rntrc": rntrc}, "")
			}
			return invokeHandler(ctx, s.deps.Transportadores, "/v1/transporte/transportadores", nil, "")
		},
	)

	s.addTool("consultar_acidentes_transito",
		"Retorna dados de acidentes rodoviários da PRF (Polícia Rodoviária Federal).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("uf", mcpgosdk.Description("Sigla do estado (ex: SP)")),
			mcpgosdk.WithString("ano", mcpgosdk.Description("Ano de referência (ex: 2025)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			uf := req.GetString("uf", "")
			ano := req.GetString("ano", "")
			query := ""
			if uf != "" {
				query += "uf=" + uf
			}
			if ano != "" {
				if query != "" {
					query += "&"
				}
				query += "ano=" + ano
			}
			return invokeHandler(ctx, s.deps.TranspAcidentes, "/v1/transporte/acidentes", nil, query)
		},
	)

	// === Comércio Exterior ===
	s.addTool("consultar_comercio_exterior",
		"Retorna dados de comércio exterior do Brasil (COMEXSTAT): exportações ou importações.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tipo",
				mcpgosdk.Description("Tipo: 'exportacoes' ou 'importacoes'. Padrão: exportacoes."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tipo := req.GetString("tipo", "exportacoes")
			if tipo == "importacoes" {
				return invokeHandler(ctx, s.deps.ComexImportacoes, "/v1/comercio/importacoes", nil, "")
			}
			return invokeHandler(ctx, s.deps.ComexExportacoes, "/v1/comercio/exportacoes", nil, "")
		},
	)

	// === Outros ===
	s.addTool("consultar_combustiveis",
		"Retorna preços de combustíveis coletados pela ANP em postos de todo o Brasil.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.Combustiveis, "/v1/energia/combustiveis", nil, "")
		},
	)

	s.addTool("consultar_planos_saude",
		"Lista operadoras e planos de saúde registrados na ANS.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.ANSPlanos, "/v1/saude/planos", nil, "")
		},
	)

	s.addTool("consultar_ipea",
		"Consulta séries temporais do IPEA por código, busca por nome, ou lista temas.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("codigo", mcpgosdk.Description("Código da série IPEA. Alternativo a 'busca'.")),
			mcpgosdk.WithString("busca", mcpgosdk.Description("Texto para buscar séries. Alternativo a 'codigo'.")),
			mcpgosdk.WithString("acao", mcpgosdk.Description("Use 'temas' para listar temas. Alternativo a codigo/busca.")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			codigo := req.GetString("codigo", "")
			busca := req.GetString("busca", "")
			acao := req.GetString("acao", "")
			if acao == "temas" {
				return invokeHandler(ctx, s.deps.IPEATemas, "/v1/ipea/temas", nil, "")
			}
			if codigo != "" {
				return invokeHandler(ctx, s.deps.IPEASerie, "/v1/ipea/serie/"+codigo, map[string]string{"codigo": codigo}, "")
			}
			if busca != "" {
				return invokeHandler(ctx, s.deps.IPEABusca, "/v1/ipea/busca", nil, "q="+busca)
			}
			return nil, fmt.Errorf("informe 'codigo', 'busca' ou acao='temas'")
		},
	)

	s.addTool("consultar_endereco",
		"Consulta endereço brasileiro por CEP (Código de Endereçamento Postal).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cep", mcpgosdk.Required(),
				mcpgosdk.Description("CEP (8 dígitos, com ou sem hífen)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cep := req.GetString("cep", "")
			return invokeHandler(ctx, s.deps.Endereco, "/v1/endereco/"+cep, map[string]string{"cep": cep}, "")
		},
	)

	s.addTool("consultar_censo_escolar",
		"Retorna dados do Censo Escolar (INEP): escolas, matrículas, docentes.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.CensoEscolar, "/v1/educacao/censo-escolar", nil, "")
		},
	)

	// === Saúde (DATASUS) ===
	s.addTool("consultar_estabelecimento_saude",
		"Busca estabelecimentos de saúde no Brasil (CNES/DATASUS). Pode buscar por código CNES individual ou listar por município (código IBGE) ou UF (código numérico).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnes",
				mcpgosdk.Description("Código CNES do estabelecimento (7 dígitos). Quando informado, retorna um único estabelecimento."),
			),
			mcpgosdk.WithString("municipio",
				mcpgosdk.Description("Código IBGE do município (6 dígitos). Para listar estabelecimentos."),
			),
			mcpgosdk.WithString("uf",
				mcpgosdk.Description("Código numérico do estado (ex: 33 para RJ, 35 para SP). Para listar estabelecimentos."),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnes := req.GetString("cnes", "")
			municipio := req.GetString("municipio", "")
			uf := req.GetString("uf", "")
			if cnes != "" {
				return invokeHandler(ctx, s.deps.DATASUSEstabelecimento, "/v1/saude/estabelecimentos/"+cnes, map[string]string{"cnes": cnes}, "")
			}
			query := ""
			if municipio != "" {
				query += "municipio=" + municipio
			}
			if uf != "" {
				if query != "" {
					query += "&"
				}
				query += "uf=" + uf
			}
			if query == "" {
				return nil, fmt.Errorf("informe 'cnes', 'municipio' ou 'uf'")
			}
			return invokeHandler(ctx, s.deps.DATASUSEstabelecimentos, "/v1/saude/estabelecimentos", nil, query)
		},
	)

	// buildHealthQuery extracts common DATASUS query params (municipio, limit) with proper URL encoding.
	buildHealthQuery := func(req mcpgosdk.CallToolRequest) string {
		q := url.Values{}
		if m := req.GetString("municipio", ""); m != "" {
			q.Set("municipio", m)
		}
		if l := req.GetString("limit", ""); l != "" {
			q.Set("limit", l)
		}
		return q.Encode()
	}

	// === Phase 4-5: Judicial, DATASUS health, Diários, Discover ===
	s.addTool("consultar_processo_judicial",
		"Consulta processo judicial pelo número unificado CNJ. Retorna dados do processo, partes, movimentações.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("numero", mcpgosdk.Required(),
				mcpgosdk.Description("Número do processo CNJ (ex: 0000000-00.0000.0.00.0000)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			numero := req.GetString("numero", "")
			return invokeHandler(ctx, s.deps.JudicialProcesso, "/v1/judicial/processo/"+url.PathEscape(numero), map[string]string{"numero": numero}, "")
		},
	)

	s.addTool("consultar_mortalidade",
		"Consulta dados de mortalidade do SIM/DATASUS (Sistema de Informação sobre Mortalidade). Filtros por município, CID, sexo, idade.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("municipio", mcpgosdk.Description("Código IBGE do município")),
			mcpgosdk.WithString("limit", mcpgosdk.Description("Número máximo de registros (padrão: 50)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.DATASUSMortalidade, "/v1/saude/mortalidade", nil, buildHealthQuery(req))
		},
	)

	s.addTool("consultar_nascimentos",
		"Consulta dados de nascidos vivos do SINASC/DATASUS. Filtros por município, sexo, peso.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("municipio", mcpgosdk.Description("Código IBGE do município")),
			mcpgosdk.WithString("limit", mcpgosdk.Description("Número máximo de registros (padrão: 50)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.DATASUSNascimentos, "/v1/saude/nascimentos", nil, buildHealthQuery(req))
		},
	)

	s.addTool("consultar_hospitais",
		"Consulta hospitais e leitos do CNES/DATASUS. Lista hospitais com quantidade de leitos por tipo.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("municipio", mcpgosdk.Description("Código IBGE do município")),
			mcpgosdk.WithString("limit", mcpgosdk.Description("Número máximo de registros (padrão: 50)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.DATASUSHospitais, "/v1/saude/hospitais", nil, buildHealthQuery(req))
		},
	)

	s.addTool("consultar_dengue",
		"Consulta notificações de dengue do SINAN/DATASUS. Dados de arboviroses por município e período.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("municipio", mcpgosdk.Description("Código IBGE do município")),
			mcpgosdk.WithString("limit", mcpgosdk.Description("Número máximo de registros (padrão: 50)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.DATASUSDengue, "/v1/saude/dengue", nil, buildHealthQuery(req))
		},
	)

	s.addTool("consultar_vacinacao",
		"Consulta doses aplicadas do PNI/DATASUS (Programa Nacional de Imunizações) por ano.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("ano", mcpgosdk.Required(),
				mcpgosdk.Description("Ano da vacinação (2020-2030)"),
			),
			mcpgosdk.WithString("limit", mcpgosdk.Description("Número máximo de registros (padrão: 50)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			ano := req.GetString("ano", "")
			var parts []string
			if l := req.GetString("limit", ""); l != "" {
				parts = append(parts, "limit="+l)
			}
			query := strings.Join(parts, "&")
			return invokeHandler(ctx, s.deps.DATASUSVacinacao, "/v1/saude/vacinacao/"+url.PathEscape(ano), map[string]string{"ano": ano}, query)
		},
	)

	s.addTool("listar_municipios_diarios",
		"Lista municípios com diários oficiais disponíveis no Querido Diário.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.DiariosMunicipios, "/v1/diarios/municipios", nil, "")
		},
	)

	s.addTool("listar_temas_diarios",
		"Lista temas de classificação disponíveis nos diários oficiais municipais.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.DiariosTemas, "/v1/diarios/temas", nil, "")
		},
	)

	s.addTool("buscar_diarios_por_tema",
		"Busca publicações em diários oficiais municipais filtradas por tema (ex: educacao, saude, meio-ambiente).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tema", mcpgosdk.Required(),
				mcpgosdk.Description("Tema de classificação (ex: educacao, saude, meio-ambiente)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tema := req.GetString("tema", "")
			return invokeHandler(ctx, s.deps.DiariosTema, "/v1/diarios/tema/"+url.PathEscape(tema), map[string]string{"tema": tema}, "")
		},
	)

	s.addTool("descobrir_casos_uso",
		"Lista casos de uso e exemplos práticos da API DataBR para diferentes setores (compliance, due diligence, saúde pública, etc).",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.DiscoverCases, "/v1/discover/cases", nil, "")
		},
	)

	// === Premium / Composite ===
	s.addTool("due_diligence_empresa",
		"Due diligence completa de empresa: cadastro + compliance + judicial + setor.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa alvo"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.DueDiligence, "/v1/empresas/"+cnpj+"/duediligence", map[string]string{"cnpj": cnpj}, "")
		},
	)

	s.addTool("perfil_completo_empresa",
		"Perfil completo de empresa: cadastro + compliance + setor + sócios.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.PerfilCompleto, "/v1/empresas/"+cnpj+"/perfil-completo", map[string]string{"cnpj": cnpj}, "")
		},
	)

	s.addTool("credito_score",
		"Calcula score de crédito de empresa (modelo 3 fatores: cadastral, compliance, judicial).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.CreditoScore, "/v1/credito/score/"+cnpj, map[string]string{"cnpj": cnpj}, "")
		},
	)

	s.addTool("panorama_economico",
		"Panorama econômico do Brasil: PIB, inflação, câmbio, emprego, comércio exterior consolidados.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			return invokeHandler(ctx, s.deps.Panorama, "/v1/economia/panorama", nil, "")
		},
	)

	s.addTool("analise_setor",
		"Análise setorial por CNPJ ou código CNAE. Inclui regulação e benchmarks.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Description("CNPJ da empresa. Alternativo a 'cnae'.")),
			mcpgosdk.WithString("cnae", mcpgosdk.Description("Código CNAE para regulação setorial. Alternativo a 'cnpj'.")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			cnae := req.GetString("cnae", "")
			if cnpj != "" {
				return invokeHandler(ctx, s.deps.SetorAnalise, "/v1/empresas/"+cnpj+"/setor", map[string]string{"cnpj": cnpj}, "")
			}
			if cnae != "" {
				return invokeHandler(ctx, s.deps.RegulacaoSetor, "/v1/setor/"+cnae+"/regulacao", map[string]string{"cnae": cnae}, "")
			}
			return nil, fmt.Errorf("informe 'cnpj' ou 'cnae'")
		},
	)

	s.addTool("analise_competicao",
		"Análise de competição no mercado por CNAE: players, concentração, barreiras.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnae", mcpgosdk.Required(),
				mcpgosdk.Description("Código CNAE do setor"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnae := req.GetString("cnae", "")
			return invokeHandler(ctx, s.deps.Competicao, "/v1/mercado/"+cnae+"/competicao", map[string]string{"cnae": cnae}, "")
		},
	)

	s.addTool("analise_mercado_trabalho",
		"Análise do mercado de trabalho por UF: emprego, salários, setores, tendências.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("uf", mcpgosdk.Required(),
				mcpgosdk.Description("Sigla do estado (ex: SP, RJ, MG)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			uf := req.GetString("uf", "")
			return invokeHandler(ctx, s.deps.MercadoTrabalho, "/v1/mercado-trabalho/"+uf+"/analise", map[string]string{"uf": uf}, "")
		},
	)

	s.addTool("analise_esg",
		"Score ESG de empresa: impacto ambiental, compliance social, governança.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.ESG, "/v1/ambiental/empresa/"+cnpj+"/esg", map[string]string{"cnpj": cnpj}, "")
		},
	)

	s.addTool("risco_ambiental",
		"Avaliação de risco ambiental de município: desmatamento, embargos, licenciamento.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("municipio", mcpgosdk.Required(),
				mcpgosdk.Description("Código IBGE do município"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			municipio := req.GetString("municipio", "")
			return invokeHandler(ctx, s.deps.RiscoAmbiental, "/v1/ambiental/risco/"+municipio, map[string]string{"municipio": municipio}, "")
		},
	)

	s.addTool("analise_litigio",
		"Análise de risco de litígio de empresa: processos, valores em disputa, tendências.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.LitigioRisco, "/v1/litigio/"+cnpj+"/risco", map[string]string{"cnpj": cnpj}, "")
		},
	)

	s.addTool("rede_influencia",
		"Mapeia rede de influência empresarial: sócios cruzados, participações, conexões políticas.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.RedeInfluencia, "/v1/rede/"+cnpj+"/influencia", map[string]string{"cnpj": cnpj}, "")
		},
	)

	s.addTool("consultar_jurisprudencia",
		"Busca decisões recentes em tribunais superiores (STF e STJ). Retorna lista de decisões com classe, relator, ementa.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("tribunal", mcpgosdk.Required(),
				mcpgosdk.Description("Tribunal: 'stf' ou 'stj'"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			tribunal := req.GetString("tribunal", "stf")
			switch tribunal {
			case "stf":
				if s.deps.JudicialSTF == nil {
					return mcpgosdk.NewToolResultText("STF decisions not available (database not connected)"), nil
				}
				return invokeHandler(ctx, s.deps.JudicialSTF, "/v1/judicial/stf", nil, "")
			case "stj":
				if s.deps.JudicialSTJ == nil {
					return mcpgosdk.NewToolResultText("STJ decisions not available (database not connected)"), nil
				}
				return invokeHandler(ctx, s.deps.JudicialSTJ, "/v1/judicial/stj", nil, "")
			default:
				return mcpgosdk.NewToolResultText("Tribunal inválido: use 'stf' ou 'stj'"), nil
			}
		},
	)

	s.addTool("perfil_municipio",
		"Perfil completo de município: demografia, economia, saúde, educação, meio ambiente.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("codigo", mcpgosdk.Required(),
				mcpgosdk.Description("Código IBGE do município (7 dígitos)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			codigo := req.GetString("codigo", "")
			return invokeHandler(ctx, s.deps.MunicipioPerfil, "/v1/municipios/"+codigo+"/perfil", map[string]string{"codigo": codigo}, "")
		},
	)

	// === Tributário ===
	s.addTool("consultar_tributos_ncm",
		"Consulta carga tributária aproximada de um produto/serviço pelo código NCM ou NBS. Retorna alíquotas federal, estadual e municipal (fonte: IBPT).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("codigo", mcpgosdk.Required(),
				mcpgosdk.Description("Código NCM (produtos, 8 dígitos) ou NBS/LC116 (serviços). Ex: 22030000, 0107"),
			),
			mcpgosdk.WithString("uf", mcpgosdk.Required(),
				mcpgosdk.Description("UF do estado (2 letras). Ex: SP, RJ, MG"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			codigo := req.GetString("codigo", "")
			uf := req.GetString("uf", "")
			return invokeHandler(ctx, s.deps.TributarioNCM, "/v1/tributario/ncm/"+codigo, map[string]string{"codigo": codigo}, "uf="+uf)
		},
	)

	s.addTool("consultar_icms",
		"Consulta alíquotas ICMS: interna de um estado, interestadual entre dois estados, ou tabela completa com todos os 27 estados.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("uf",
				mcpgosdk.Description("UF para consulta de alíquota interna (2 letras). Ex: SP"),
			),
			mcpgosdk.WithString("origem",
				mcpgosdk.Description("UF de origem para cálculo interestadual. Ex: SP"),
			),
			mcpgosdk.WithString("destino",
				mcpgosdk.Description("UF de destino para cálculo interestadual. Ex: MA"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			uf := req.GetString("uf", "")
			origem := req.GetString("origem", "")
			destino := req.GetString("destino", "")
			if uf != "" {
				return invokeHandler(ctx, s.deps.TributarioICMS, "/v1/tributario/icms/"+uf, map[string]string{"uf": uf}, "")
			}
			qp := ""
			if origem != "" && destino != "" {
				qp = "origem=" + origem + "&destino=" + destino
			}
			return invokeHandler(ctx, s.deps.TributarioICMS, "/v1/tributario/icms", nil, qp)
		},
	)

	// === Transparência Federal (new) ===
	s.addTool("consultar_pgfn",
		"Consulta dívida ativa na Procuradoria-Geral da Fazenda Nacional (PGFN)",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj",
				mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ do devedor (somente dígitos)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.TranspPGFN, "/v1/transparencia/pgfn", nil, "cnpj="+cnpj)
		},
	)

	s.addTool("consultar_pep",
		"Consulta Pessoas Expostas Politicamente (PEP) pelo nome",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("nome",
				mcpgosdk.Required(),
				mcpgosdk.Description("Nome da pessoa para busca"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			nome := req.GetString("nome", "")
			return invokeHandler(ctx, s.deps.TranspPEP, "/v1/transparencia/pep", nil, "nome="+url.QueryEscape(nome))
		},
	)

	s.addTool("consultar_leniencias",
		"Consulta acordos de leniência firmados pelo CNPJ",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj",
				mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.TranspLeniencias, "/v1/transparencia/leniencias", nil, "cnpj="+cnpj)
		},
	)

	s.addTool("consultar_renuncias_fiscais",
		"Consulta renúncias fiscais registradas no Portal da Transparência",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj",
				mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ do beneficiário"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.TranspRenuncias, "/v1/transparencia/renuncias", nil, "cnpj="+cnpj)
		},
	)

	// === BNDES ===
	s.addTool("consultar_bndes",
		"Consulta operações de crédito do BNDES para um CNPJ",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj",
				mcpgosdk.Required(),
				mcpgosdk.Description("CNPJ da empresa beneficiária"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return invokeHandler(ctx, s.deps.BNDESOperacoes, "/v1/bndes/"+cnpj+"/operacoes", map[string]string{"cnpj": cnpj}, "")
		},
	)

	// === TSE Filiados ===
	s.addTool("consultar_filiados_tse",
		"Consulta filiados partidários por UF (estado) via TSE",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("uf",
				mcpgosdk.Required(),
				mcpgosdk.Description("Sigla do estado (ex: SP, RJ)"),
			),
			mcpgosdk.WithString("n",
				mcpgosdk.Description("Número máximo de registros (padrão: 100)"),
			),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			uf := req.GetString("uf", "")
			n := req.GetString("n", "100")
			query := "uf=" + uf
			if n != "" && n != "100" {
				query += "&n=" + n
			} else {
				query += "&n=100"
			}
			return invokeHandler(ctx, s.deps.TSEFiliados, "/v1/eleicoes/filiados", nil, query)
		},
	)
}

// addTool registers a tool and records its name for introspection.
func (s *Server) addTool(name, desc string, opts []mcpgosdk.ToolOption, handler server.ToolHandlerFunc) {
	allOpts := append([]mcpgosdk.ToolOption{mcpgosdk.WithDescription(desc)}, opts...)
	s.mcpServer.AddTool(mcpgosdk.NewTool(name, allOpts...), handler)
	s.tools = append(s.tools, name)
}

// extractJSON extracts parsed JSON from a tool result, for aggregation in composite tools.
func extractJSON(result *mcpgosdk.CallToolResult) any {
	if result == nil {
		return nil
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcpgosdk.TextContent); ok {
			var m any
			if err := json.Unmarshal([]byte(tc.Text), &m); err == nil {
				return m
			}
		}
	}
	return nil
}
