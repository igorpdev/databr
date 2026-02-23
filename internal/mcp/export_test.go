package mcp

import (
	"context"
	"net/http"

	mcpgosdk "github.com/mark3labs/mcp-go/mcp"
)

// InvokeHandlerForTest exposes invokeHandler for external tests.
func InvokeHandlerForTest(ctx context.Context, handler http.HandlerFunc, path string, chiParams map[string]string, query string) (*mcpgosdk.CallToolResult, error) {
	return invokeHandler(ctx, handler, path, chiParams, query)
}
