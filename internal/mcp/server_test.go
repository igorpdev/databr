package mcp_test

import (
	"testing"

	"github.com/databr/api/internal/mcp"
)

func TestMCPServer_NotNil(t *testing.T) {
	srv := mcp.NewServer("http://localhost:8080")
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestMCPServer_ToolsRegistered(t *testing.T) {
	srv := mcp.NewServer("http://localhost:8080")
	tools := srv.Tools()

	required := []string{
		"consultar_empresa",
		"verificar_compliance",
		"cotacao_cambio",
		"indicadores_macro",
		"buscar_processos_judiciais",
		"buscar_diario_oficial",
		"consultar_orcamento",
		"consultar_tcu_certidao",
		"cotacao_acoes",
		"consultar_deputados",
		"buscar_licitacao",
		"consultar_tarifas_energia",
		"consultar_medicamento",
	}

	toolSet := make(map[string]bool, len(tools))
	for _, t := range tools {
		toolSet[t] = true
	}

	for _, name := range required {
		if !toolSet[name] {
			t.Errorf("MCP tool %q not registered", name)
		}
	}
}

func TestMCPServer_BaseURLRequired(t *testing.T) {
	// Server must include a base URL so tools know where to call
	srv := mcp.NewServer("")
	if srv == nil {
		t.Fatal("NewServer returned nil even with empty URL")
	}
	// Empty URL should be handled gracefully (not panic)
}
