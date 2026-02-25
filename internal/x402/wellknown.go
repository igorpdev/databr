package x402

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"
)

// paramExamples maps chi route parameter names to concrete example values.
// These are used to build full URLs for the x402scan discovery document:
// x402scan probes each URL and expects a 402 response — the middleware
// intercepts before the handler runs, so any valid path segment works.
var paramExamples = map[string]string{
	"cnpj":      "00000000000191", // Banco do Brasil
	"moeda":     "USD",
	"ticker":    "PETR4",
	"serie":     "11", // SGS: CDI
	"uf":        "SP",
	"ibge":      "3550308", // São Paulo
	"id":        "1",
	"codigo":    "6201500", // CNAE: Desenvolvimento de programas de computador
	"tema":      "educacao",
	"protocolo": "1234567",
	"municipio": "3550308",
	"numero":    "10001",
	"doc":       "00000000000191",
	"cnae":      "6201500",
	"cep":       "01310100", // Av. Paulista
	"registro":  "1234567890",
	"cnes":      "2077485",
	"rntrc":     "1234567",
	"prefixo":   "PR-XXX",
	"ano":       "2024",
	"cpf_cnpj":  "00000000000191",
	"cpf":       "12345678901",
}

// exampleURL replaces chi route parameters ({param}) with concrete example values
// so the resulting URL is a real, testable path that returns 402 via the middleware.
func exampleURL(pattern string) string {
	result := pattern
	for param, example := range paramExamples {
		result = strings.ReplaceAll(result, "{"+param+"}", example)
	}
	return result
}

// WellKnownEndpoint describes a single payable endpoint for agent discovery.
type WellKnownEndpoint struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
	Amount      string `json:"amount"`    // atomic USDC units (6 decimals)
	PriceUSDC   string `json:"priceUSDC"` // human-readable: "0.003"
}

// WellKnownResponse is the body returned by GET /.well-known/x402.
//
// It satisfies two discovery formats simultaneously:
//   - x402scan format: "version" (int 1) + "resources" (array of full URLs) + "ownershipProofs"
//   - DataBR agent format: "x402Version" + "endpoints" with pricing metadata
type WellKnownResponse struct {
	// x402scan discovery format (required by Merit-Systems/x402scan)
	Version         int      `json:"version"`                   // always 1
	Resources       []string `json:"resources"`                 // full URLs that return 402 when probed
	OwnershipProofs []string `json:"ownershipProofs,omitempty"` // EIP-191 sigs of origin URL per payTo address

	// DataBR extended format (for x402-aware AI agents)
	X402Version int                 `json:"x402Version"` // always 2
	PayTo       string              `json:"payTo"`
	Network     string              `json:"network"`
	Asset       string              `json:"asset"`
	Endpoints   []WellKnownEndpoint `json:"endpoints"`
}

// WellKnownHandler returns an HTTP handler for GET /.well-known/x402.
// The endpoint is public (no x402 middleware) — it is the discovery document.
func WellKnownHandler(cfg MiddlewareConfig) http.HandlerFunc {
	asset := usdcAssets[cfg.Network]
	if asset == "" {
		asset = usdcAssets["eip155:84532"]
	}

	baseURL := strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "https://databr.api.br"
	}

	// Build endpoint and resources lists once at startup (priceTable is static).
	endpoints := make([]WellKnownEndpoint, 0, len(priceTable))
	resources := make([]string, 0, len(priceTable))

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
		// Build a concrete URL with example params for x402scan to probe.
		resources = append(resources, baseURL+exampleURL(pattern))
	}

	// Sort for stable, readable output.
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Path < endpoints[j].Path
	})
	sort.Strings(resources)

	// Read ownership proofs from env (comma-separated hex signatures).
	// Generate with: cast wallet sign --no-hash "https://databr.api.br" --private-key <KEY>
	var ownershipProofs []string
	if raw := os.Getenv("X402_OWNERSHIP_PROOF"); raw != "" {
		for _, sig := range strings.Split(raw, ",") {
			if sig = strings.TrimSpace(sig); sig != "" {
				ownershipProofs = append(ownershipProofs, sig)
			}
		}
	}

	resp := WellKnownResponse{
		Version:         1,
		Resources:       resources,
		OwnershipProofs: ownershipProofs,
		X402Version:     2,
		PayTo:           cfg.WalletAddress,
		Network:         cfg.Network,
		Asset:           asset,
		Endpoints:       endpoints,
	}
	body, _ := json.Marshal(resp)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(body) //nolint:errcheck
	}
}
