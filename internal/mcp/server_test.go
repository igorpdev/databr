package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/databr/api/internal/mcp"
	"github.com/go-chi/chi/v5"
	mcpgosdk "github.com/mark3labs/mcp-go/mcp"
)

func TestMCPServer_NotNil(t *testing.T) {
	srv := mcp.NewServer(&mcp.HandlerDeps{})
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestMCPServer_NilDeps(t *testing.T) {
	srv := mcp.NewServer(nil)
	if srv == nil {
		t.Fatal("NewServer returned nil with nil deps")
	}
}

func TestMCPServer_ToolsRegistered(t *testing.T) {
	srv := mcp.NewServer(&mcp.HandlerDeps{})
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

func TestInvokeHandler_ChiParams(t *testing.T) {
	// Handler that reads chi.URLParam("cnpj") and echoes it back.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cnpj := chi.URLParam(r, "cnpj")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"cnpj": cnpj})
	})

	result, err := mcp.InvokeHandlerForTest(
		context.Background(),
		handler,
		"/v1/empresas/12345678000190",
		map[string]string{"cnpj": "12345678000190"},
		"",
	)
	if err != nil {
		t.Fatalf("invokeHandler returned error: %v", err)
	}

	// Parse the tool result text and verify the CNPJ was injected.
	var resp map[string]string
	for _, c := range result.Content {
		if tc, ok := c.(mcpgosdk.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), &resp); err == nil {
				break
			}
		}
	}
	if resp["cnpj"] != "12345678000190" {
		t.Errorf("expected cnpj=12345678000190, got %q", resp["cnpj"])
	}
}

func TestInvokeHandler_NilHandler(t *testing.T) {
	// Nil handler should return an error, not panic.
	_, err := mcp.InvokeHandlerForTest(
		context.Background(),
		nil,
		"/v1/mercado/acoes/PETR4",
		map[string]string{"ticker": "PETR4"},
		"",
	)
	if err == nil {
		t.Fatal("expected error from nil handler, got nil")
	}
}

func TestInvokeHandler_ErrorStatus(t *testing.T) {
	// Handler that returns 404.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	_, err := mcp.InvokeHandlerForTest(
		context.Background(),
		handler,
		"/v1/bcb/selic",
		nil,
		"",
	)
	if err == nil {
		t.Fatal("expected error from 404 handler, got nil")
	}
}
