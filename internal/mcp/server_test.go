package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/mcp"
	x402pkg "github.com/databr/api/internal/x402"
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

// TestPerToolMiddleware_GET verifies that GET requests pass free (streaming heartbeat).
func TestPerToolMiddleware_GET(t *testing.T) {
	cfg := x402pkg.MiddlewareConfig{}

	called := false
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()

	mcp.NewPerToolMiddleware(cfg)(capture).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected capture handler to be called for GET, but it was not")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET, got %d", rec.Code)
	}
}

// TestToolPrices_AllToolsCovered verifies every registered MCP tool has an entry in ToolPrices.
func TestToolPrices_AllToolsCovered(t *testing.T) {
	srv := mcp.NewServer(&mcp.HandlerDeps{})
	for _, name := range srv.Tools() {
		if _, ok := mcp.ToolPrices[name]; !ok {
			t.Errorf("tool %q has no entry in mcp.ToolPrices", name)
		}
	}
}

// TestPerToolMiddleware_Initialize verifies that initialize requests pass free (HTTP 200).
func TestPerToolMiddleware_Initialize(t *testing.T) {
	// Dev config: no wallet address → price injector only, no x402 gate.
	cfg := x402pkg.MiddlewareConfig{}

	called := false
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mcp.NewPerToolMiddleware(cfg)(capture).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected capture handler to be called for initialize, but it was not")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for initialize, got %d", rec.Code)
	}
}

// TestPerToolMiddleware_ToolsCall verifies that tools/call injects the correct per-tool price.
func TestPerToolMiddleware_ToolsCall(t *testing.T) {
	// Dev config: no wallet address → price injector, no real x402 gate.
	cfg := x402pkg.MiddlewareConfig{}

	var capturedPrice string
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPrice = x402pkg.PriceFromRequest(r)
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		tool  string
		price string
	}{
		{"consultar_empresa", "0.003"},
		{"buscar_processos_judiciais", "0.015"},
		{"verificar_compliance", "0.010"},
		{"cotacao_acoes", "0.005"},
	}

	for _, tc := range tests {
		t.Run(tc.tool, func(t *testing.T) {
			capturedPrice = ""
			body, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "tools/call",
				"params":  map[string]string{"name": tc.tool},
			})
			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			mcp.NewPerToolMiddleware(cfg)(capture).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
			if capturedPrice != tc.price {
				t.Errorf("tool %q: expected price %q, got %q", tc.tool, tc.price, capturedPrice)
			}
		})
	}
}
