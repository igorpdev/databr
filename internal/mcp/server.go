// Package mcp implements the DataBR MCP Server.
// Tools here are thin proxies over the DataBR REST API, enabling
// Claude and other MCP-compatible AI agents to access Brazilian public data.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	mcpgosdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the mcp-go server with DataBR tool registrations.
type Server struct {
	mcpServer *server.MCPServer
	baseURL   string
	tools     []string
}

// NewServer creates a DataBR MCP Server that proxies to the REST API at baseURL.
func NewServer(baseURL string) *Server {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	s := &Server{
		mcpServer: server.NewMCPServer(
			"DataBR",
			"1.0.0",
			server.WithToolCapabilities(true),
		),
		baseURL: baseURL,
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
			return s.callAPI(ctx, fmt.Sprintf("/v1/empresas/%s", cnpj))
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
			return s.callAPI(ctx, fmt.Sprintf("/v1/compliance/%s", cnpj))
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
			return s.callAPI(ctx, fmt.Sprintf("/v1/bcb/cambio/%s", moeda))
		},
	)

	s.addTool("indicadores_macro",
		"Retorna indicadores macroeconômicos do Brasil: IPCA (inflação), Selic (juros), PIB e câmbio USD.",
		[]mcpgosdk.ToolOption{},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			selic, err := s.callAPI(ctx, "/v1/bcb/selic")
			if err != nil {
				return nil, err
			}
			ipca, _ := s.callAPI(ctx, "/v1/economia/ipca")
			pib, _ := s.callAPI(ctx, "/v1/economia/pib")
			cambio, _ := s.callAPI(ctx, "/v1/bcb/cambio/USD")

			result := map[string]any{
				"selic":  extractData(selic),
				"ipca":   extractData(ipca),
				"pib":    extractData(pib),
				"cambio": extractData(cambio),
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
			return s.callAPI(ctx, fmt.Sprintf("/v1/judicial/processos/%s", doc))
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
			path := fmt.Sprintf("/v1/dou/busca?q=%s", q)
			if uf != "" {
				path += "&uf=" + uf
			}
			return s.callAPI(ctx, path)
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
			path := "/v1/orcamento/despesas?ano=" + ano
			if orgao != "" {
				path += "&orgao=" + orgao
			}
			return s.callAPI(ctx, path)
		},
	)

	s.addTool("consultar_tcu_certidao",
		"Verifica certidão de regularidade de empresa no TCU (Tribunal de Contas da União).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(), mcpgosdk.Description("CNPJ da empresa")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return s.callAPI(ctx, fmt.Sprintf("/v1/tcu/certidao/%s", cnpj))
		},
	)

	s.addTool("cotacao_acoes",
		"Retorna cotação de ação na B3 (Bolsa de Valores do Brasil) pelo ticker.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("ticker", mcpgosdk.Required(), mcpgosdk.Description("Ticker da ação (ex: PETR4, VALE3, ITUB4)")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			ticker := req.GetString("ticker", "")
			return s.callAPI(ctx, fmt.Sprintf("/v1/mercado/acoes/%s", ticker))
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
			path := "/v1/legislativo/deputados?"
			if uf != "" {
				path += "uf=" + uf + "&"
			}
			if partido != "" {
				path += "partido=" + partido
			}
			return s.callAPI(ctx, path)
		},
	)

	s.addTool("buscar_licitacao",
		"Busca licitações e contratações públicas no PNCP (Portal Nacional de Contratações Públicas).",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("cnpj", mcpgosdk.Required(), mcpgosdk.Description("CNPJ do órgão contratante")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			cnpj := req.GetString("cnpj", "")
			return s.callAPI(ctx, fmt.Sprintf("/v1/pncp/orgaos?cnpj=%s", cnpj))
		},
	)

	s.addTool("consultar_tarifas_energia",
		"Retorna tarifas de energia elétrica da ANEEL por distribuidora.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("distribuidora", mcpgosdk.Description("Nome da distribuidora (ex: ENEL, CEMIG). Opcional.")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			dist := req.GetString("distribuidora", "")
			path := "/v1/energia/tarifas"
			if dist != "" {
				path += "?distribuidora=" + dist
			}
			return s.callAPI(ctx, path)
		},
	)

	s.addTool("consultar_medicamento",
		"Busca medicamento registrado na ANVISA pelo número de registro.",
		[]mcpgosdk.ToolOption{
			mcpgosdk.WithString("registro", mcpgosdk.Required(), mcpgosdk.Description("Número de registro ANVISA do medicamento")),
		},
		func(ctx context.Context, req mcpgosdk.CallToolRequest) (*mcpgosdk.CallToolResult, error) {
			registro := req.GetString("registro", "")
			return s.callAPI(ctx, fmt.Sprintf("/v1/saude/medicamentos/%s", registro))
		},
	)
}

// addTool registers a tool and records its name for introspection.
func (s *Server) addTool(name, desc string, opts []mcpgosdk.ToolOption, handler server.ToolHandlerFunc) {
	allOpts := append([]mcpgosdk.ToolOption{mcpgosdk.WithDescription(desc)}, opts...)
	s.mcpServer.AddTool(mcpgosdk.NewTool(name, allOpts...), handler)
	s.tools = append(s.tools, name)
}

// callAPI makes an HTTP GET to the DataBR REST API and returns a tool result.
func (s *Server) callAPI(ctx context.Context, path string) (*mcpgosdk.CallToolResult, error) {
	url := s.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp: build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: call %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcp: read response: %w", err)
	}

	return mcpgosdk.NewToolResultText(string(body)), nil
}

// extractData extracts the "data" field from a tool result JSON, for aggregation.
func extractData(result *mcpgosdk.CallToolResult) any {
	if result == nil {
		return nil
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcpgosdk.TextContent); ok {
			var m map[string]any
			if err := json.Unmarshal([]byte(tc.Text), &m); err == nil {
				if d, ok := m["data"]; ok {
					return d
				}
			}
		}
	}
	return nil
}
