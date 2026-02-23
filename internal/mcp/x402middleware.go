package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	x402pkg "github.com/databr/api/internal/x402"
)

// jsonRPCRequest is the minimal JSON-RPC 2.0 structure needed to identify tool calls.
type jsonRPCRequest struct {
	Method string `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
}

// NewPerToolMiddleware returns a middleware that applies x402 payment gates per MCP tool call.
//
// Request routing:
//   - GET (streaming / SSE heartbeat) — passes free
//   - POST method != "tools/call" (initialize, tools/list, notifications/...) — passes free
//   - POST method == "tools/call" — x402 gate with the tool-specific price from ToolPrices
//
// In production (WALLET_ADDRESS set) a real x402 payment is required.
// In dev mode (no wallet) the price is injected into context so handlers can read it.
func NewPerToolMiddleware(cfg x402pkg.MiddlewareConfig) func(http.Handler) http.Handler {
	isProd := cfg.WalletAddress != ""

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// GET requests (streaming heartbeat / SSE fallback) pass free.
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			// Buffer the body (max 1 MB) and re-inject so the MCP handler can read it.
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			// Parse the JSON-RPC envelope to find method and tool name.
			var rpc jsonRPCRequest
			if err := json.Unmarshal(body, &rpc); err != nil || rpc.Method != "tools/call" {
				// Non-tool-call messages (initialize, tools/list, notifications) pass free.
				next.ServeHTTP(w, r)
				return
			}

			// Determine price: use tool-specific price, fall back to $0.003.
			price, ok := ToolPrices[rpc.Params.Name]
			if !ok {
				price = "0.003"
			}

			// Apply x402 gate (production) or price injection (dev).
			if isProd {
				x402pkg.NewPricedMiddleware(cfg, price)(next).ServeHTTP(w, r)
			} else {
				x402pkg.PriceInjectorMiddleware(price)(next).ServeHTTP(w, r)
			}
		})
	}
}
