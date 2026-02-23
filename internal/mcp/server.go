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
