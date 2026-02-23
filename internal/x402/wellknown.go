package x402

import (
	"encoding/json"
	"net/http"
	"sort"
)

// WellKnownEndpoint describes a single payable endpoint for discovery.
type WellKnownEndpoint struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
	Amount      string `json:"amount"`    // atomic USDC units (6 decimals)
	PriceUSDC   string `json:"priceUSDC"` // human-readable: "0.003"
}

// WellKnownResponse is the body returned by GET /.well-known/x402.
// It allows x402-aware agents to discover all payable endpoints,
// their prices, and the payment destination before making any request.
type WellKnownResponse struct {
	X402Version int                  `json:"x402Version"`
	PayTo       string               `json:"payTo"`
	Network     string               `json:"network"`
	Asset       string               `json:"asset"`
	Endpoints   []WellKnownEndpoint  `json:"endpoints"`
}

// WellKnownHandler returns an HTTP handler for GET /.well-known/x402.
// The endpoint is public (no x402 middleware) — it is the discovery document.
func WellKnownHandler(cfg MiddlewareConfig) http.HandlerFunc {
	asset := usdcAssets[cfg.Network]
	if asset == "" {
		asset = usdcAssets["eip155:84532"]
	}

	// Build endpoint list once at startup (priceTable is static).
	endpoints := make([]WellKnownEndpoint, 0, len(priceTable))
	for pattern, price := range priceTable {
		desc, _ := RouteMeta(pattern)
		method := "GET"
		if pattern == "/v1/carteira/risco" {
			method = "POST"
		}
		endpoints = append(endpoints, WellKnownEndpoint{
			Path:        pattern,
			Method:      method,
			Description: desc,
			Amount:      USDCToAtomicUnits(price),
			PriceUSDC:   price,
		})
	}
	// Sort by path for stable, readable output.
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Path < endpoints[j].Path
	})

	resp := WellKnownResponse{
		X402Version: 2,
		PayTo:       cfg.WalletAddress,
		Network:     cfg.Network,
		Asset:       asset,
		Endpoints:   endpoints,
	}
	body, _ := json.Marshal(resp)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(body) //nolint:errcheck
	}
}
