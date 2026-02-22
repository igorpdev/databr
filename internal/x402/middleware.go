package x402

import (
	"net/http"

	x402sdk "github.com/mark3labs/x402-go"
	x402http "github.com/mark3labs/x402-go/http"
)

// MiddlewareConfig holds configuration for the x402 payment middleware.
type MiddlewareConfig struct {
	// WalletAddress is the address that receives USDC payments.
	WalletAddress string

	// FacilitatorURL is the x402 facilitator endpoint.
	// Testnet: https://facilitator.x402.rs
	// Mainnet: https://api.cdp.coinbase.com/platform/v2/x402
	FacilitatorURL string

	// Network is the blockchain network name: "base-sepolia" or "base".
	Network string
}

// NewPricedMiddleware creates a Chi-compatible x402 payment middleware
// for the given fixed USDC price (e.g. "0.001").
// Use separate middleware instances per price tier, applied to route groups.
func NewPricedMiddleware(cfg MiddlewareConfig, priceUSDC string) func(http.Handler) http.Handler {
	chainConfig := resolveChain(cfg.Network)

	requirement, err := x402sdk.NewUSDCPaymentRequirement(x402sdk.USDCRequirementConfig{
		Chain:             chainConfig,
		Amount:            priceUSDC,
		RecipientAddress:  cfg.WalletAddress,
		Description:       "DataBR — dados públicos brasileiros",
		MaxTimeoutSeconds: 300,
	})
	if err != nil {
		// This only fails if parameters are malformed (e.g. empty wallet address).
		// In production this would panic at startup, which is intentional.
		panic("x402: invalid payment requirement config: " + err.Error())
	}

	return x402http.NewX402Middleware(&x402http.Config{
		FacilitatorURL:      cfg.FacilitatorURL,
		PaymentRequirements: []x402sdk.PaymentRequirement{requirement},
		VerifyOnly:          false,
	})
}

// HealthBypassMiddleware wraps another middleware and skips x402 for public paths
// (e.g. /health, /metrics). Must wrap the payment middleware, not the handler.
func HealthBypassMiddleware(paymentMiddleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			paymentMiddleware(next).ServeHTTP(w, r)
		})
	}
}

// resolveChain maps a network name string to the x402-go ChainConfig.
func resolveChain(network string) x402sdk.ChainConfig {
	switch network {
	case "base":
		return x402sdk.BaseMainnet
	case "base-sepolia":
		return x402sdk.BaseSepolia
	case "polygon":
		return x402sdk.PolygonMainnet
	case "polygon-amoy":
		return x402sdk.PolygonAmoy
	default:
		return x402sdk.BaseSepolia // safe default for tests and development
	}
}
