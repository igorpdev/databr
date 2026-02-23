package x402

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	x402types "github.com/coinbase/x402/go/types"
	fachttp "github.com/coinbase/x402/go/http"
)

// priceContextKey is the context key for the USDC price injected by x402 middleware.
type priceCtxKey struct{}

// PriceFromRequest returns the USDC price string set by the x402 middleware.
// Handlers use this instead of hardcoding prices, so pricing is centralized in main.go.
// Returns DefaultPrice if no middleware is present (e.g. in unit tests without middleware).
func PriceFromRequest(r *http.Request) string {
	if v, ok := r.Context().Value(priceCtxKey{}).(string); ok {
		return v
	}
	return DefaultPrice
}

// InjectPrice returns a copy of r with the given USDC price in its context.
// Used in tests to simulate x402 middleware without the full payment flow.
func InjectPrice(r *http.Request, price string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), priceCtxKey{}, price))
}

// PriceInjectorMiddleware returns a middleware that injects the USDC price into the
// request context without requiring payment. Used in dev mode when x402 is disabled.
func PriceInjectorMiddleware(priceUSDC string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), priceCtxKey{}, priceUSDC)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// USDC contract addresses per CAIP-2 network identifier.
var usdcAssets = map[string]string{
	"eip155:8453":  "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // Base mainnet
	"eip155:84532": "0x036CbD53842c5426634e7929541eC2318f3dCF7e", // Base Sepolia
}

// MiddlewareConfig holds configuration for the x402 payment middleware.
type MiddlewareConfig struct {
	// WalletAddress is the address that receives USDC payments.
	WalletAddress string

	// FacilitatorURL is the x402 facilitator endpoint.
	// Testnet: https://x402.org/facilitator
	// Mainnet: https://api.cdp.coinbase.com/platform/v2/x402
	FacilitatorURL string

	// Network is the CAIP-2 chain identifier: "eip155:8453" (mainnet) or "eip155:84532" (testnet).
	Network string

	// CDPKeyID is the Coinbase Developer Platform API key ID (UUID from the portal).
	// Required only for the CDP mainnet facilitator.
	CDPKeyID string

	// CDPKeySecret is the base64-encoded private key from the CDP portal.
	// Supports Ed25519 (64-byte raw), Ed25519 seed (32-byte), PKCS8, and SEC1/EC formats.
	CDPKeySecret string
}

// NewPricedMiddleware creates a Chi-compatible x402 payment middleware
// for the given fixed USDC price (e.g. "0.001").
// Use separate middleware instances per price tier, applied to route groups.
func NewPricedMiddleware(cfg MiddlewareConfig, priceUSDC string) func(http.Handler) http.Handler {
	asset := usdcAssets[cfg.Network]
	if asset == "" {
		asset = usdcAssets["eip155:84532"] // default testnet
	}

	baseReq := x402types.PaymentRequirements{
		Scheme:            "exact",
		Network:           cfg.Network,
		Asset:             asset,
		Amount:            USDCToAtomicUnits(priceUSDC),
		PayTo:             cfg.WalletAddress,
		MaxTimeoutSeconds: 300,
	}

	facURL := cfg.FacilitatorURL
	if facURL == "" {
		facURL = "https://x402.org/facilitator"
	}

	var authProvider fachttp.AuthProvider
	if cfg.CDPKeyID != "" && cfg.CDPKeySecret != "" {
		auth, err := newCDPAuthProvider(cfg.CDPKeyID, cfg.CDPKeySecret, facURL)
		if err != nil {
			panic("x402: invalid CDP credentials: " + err.Error())
		}
		authProvider = auth
		log.Printf("x402: CDP mainnet facilitator auth enabled (key=%s…)", cfg.CDPKeyID[:8])
	}

	facClient := fachttp.NewHTTPFacilitatorClient(&fachttp.FacilitatorConfig{
		URL:          facURL,
		AuthProvider: authProvider,
		Timeout:      30 * time.Second,
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Inject price into context so handlers can read it via PriceFromRequest.
			ctx := context.WithValue(r.Context(), priceCtxKey{}, priceUSDC)
			r = r.WithContext(ctx)

			payloadBytes := extractPaymentHeader(r)
			if payloadBytes == nil {
				write402Response(w, r, baseReq)
				return
			}

			// Build reqBytes per-request with discovery metadata so the CDP
			// facilitator can extract it for Bazaar indexing.
			reqBytes := buildRequirementsBytes(r, baseReq)

			verifyResp, err := facClient.Verify(r.Context(), payloadBytes, reqBytes)
			if err != nil {
				log.Printf("x402: verify error: %v", err)
				http.Error(w, `{"error":"payment verification failed"}`, http.StatusBadGateway)
				return
			}
			if !verifyResp.IsValid {
				log.Printf("x402: payment invalid: %s", verifyResp.InvalidReason)
				write402Response(w, r, baseReq)
				return
			}

			settleResp, err := facClient.Settle(r.Context(), payloadBytes, reqBytes)
			if err != nil {
				log.Printf("x402: settle error: %v", err)
				http.Error(w, `{"error":"payment settlement failed"}`, http.StatusBadGateway)
				return
			}

			if settleResp.Success {
				respJSON, _ := json.Marshal(settleResp)
				w.Header().Set("X-PAYMENT-RESPONSE", string(respJSON))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractPaymentHeader reads the payment proof from the request.
// Supports both V2 (Payment-Signature, base64) and V1 (X-Payment, raw JSON).
func extractPaymentHeader(r *http.Request) []byte {
	// V2: Payment-Signature header (base64-encoded JSON)
	if sig := r.Header.Get("Payment-Signature"); sig != "" {
		if decoded, err := base64.StdEncoding.DecodeString(sig); err == nil {
			return decoded
		}
		if decoded, err := base64.RawStdEncoding.DecodeString(sig); err == nil {
			return decoded
		}
	}
	// V1: X-Payment header (raw JSON)
	if payment := r.Header.Get("X-Payment"); payment != "" {
		return []byte(payment)
	}
	return nil
}

// buildRequirementsBytes creates the requirements JSON sent to the CDP facilitator for
// verify/settle. It includes Bazaar discovery fields (description, mimeType, outputSchema)
// so the facilitator can extract them for Bazaar indexing via ExtractDiscoveredResourceFromPaymentPayload.
func buildRequirementsBytes(r *http.Request, base x402types.PaymentRequirements) []byte {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	resourceURL := scheme + "://" + r.Host + r.URL.Path

	meta, ok := matchRouteMeta(r.URL.Path)
	if !ok {
		meta = routeMetaEntry{"DataBR — dados públicos brasileiros", "application/json"}
	}

	method := "GET"
	if r.Method == http.MethodPost {
		method = "POST"
	}

	req := map[string]interface{}{
		"scheme":            base.Scheme,
		"network":           base.Network,
		"asset":             base.Asset,
		"amount":            base.Amount,
		"payTo":             base.PayTo,
		"maxTimeoutSeconds": base.MaxTimeoutSeconds,
		"maxAmountRequired": base.Amount,
		"resource":          resourceURL,
		"description":       meta.description,
		"mimeType":          meta.mimeType,
		"discoverable":      true,
		"outputSchema": map[string]interface{}{
			"input": map[string]interface{}{
				"discoverable": true,
				"method":       method,
				"type":         "http",
			},
			"output": map[string]interface{}{
				"type": "object",
			},
		},
	}
	b, _ := json.Marshal(req)
	return b
}

// write402Response writes a V2 PaymentRequired JSON response with Bazaar discovery fields.
// The Bazaar indexer reads description, mimeType, discoverable, and outputSchema from
// each accepts[] item (V1-style), so we include those alongside V2 fields for compatibility.
func write402Response(w http.ResponseWriter, r *http.Request, req x402types.PaymentRequirements) {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	resourceURL := scheme + "://" + r.Host + r.URL.Path

	meta, ok := matchRouteMeta(r.URL.Path)
	if !ok {
		meta = routeMetaEntry{"DataBR — dados públicos brasileiros", "application/json"}
	}

	method := "GET"
	if r.Method == http.MethodPost {
		method = "POST"
	}

	// Build accepts item with V2 payment fields + V1-style discovery fields.
	// The V2 client reads scheme/network/amount/payTo/asset; the Bazaar indexer
	// reads description/mimeType/discoverable/outputSchema/resource/maxAmountRequired.
	acceptsItem := map[string]interface{}{
		// V2 payment fields
		"scheme":            req.Scheme,
		"network":           req.Network,
		"asset":             req.Asset,
		"amount":            req.Amount,
		"payTo":             req.PayTo,
		"maxTimeoutSeconds": req.MaxTimeoutSeconds,
		// V1 compat for Bazaar indexer
		"maxAmountRequired": req.Amount,
		"resource":          resourceURL,
		"description":       meta.description,
		"mimeType":          meta.mimeType,
		"discoverable":      true,
		"outputSchema": map[string]interface{}{
			"input": map[string]interface{}{
				"discoverable": true,
				"method":       method,
				"type":         "http",
			},
			"output": map[string]interface{}{
				"type": "object",
			},
		},
	}

	// V2 extensions.bazaar with proper info/schema format.
	// The x402 client copies Extensions into PaymentPayload.Extensions,
	// and the CDP facilitator extracts from payloadBytes.extensions["bazaar"].info.
	bazaarExtension := map[string]interface{}{
		"info": map[string]interface{}{
			"input": map[string]interface{}{
				"type":   "http",
				"method": method,
			},
		},
		"schema": map[string]interface{}{},
	}

	resp := map[string]interface{}{
		"x402Version": 2,
		"resource": map[string]interface{}{
			"url":         resourceURL,
			"description": meta.description,
			"mimeType":    meta.mimeType,
		},
		"accepts": []interface{}{acceptsItem},
		"extensions": map[string]interface{}{
			"bazaar": bazaarExtension,
		},
	}

	body, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired)
	w.Write(body) //nolint:errcheck
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

// ---- CDP JWT Authentication ----

// cdpAuthProvider implements fachttp.AuthProvider for Coinbase Developer Platform JWT auth.
type cdpAuthProvider struct {
	keyID       string
	privateKey  interface{} // *ecdsa.PrivateKey or ed25519.PrivateKey
	algorithm   string      // "ES256" or "EdDSA"
	facHost     string      // e.g. "api.cdp.coinbase.com"
	facBasePath string      // e.g. "/platform/v2/x402"
}

func newCDPAuthProvider(keyID, keySecretB64, facilitatorURL string) (*cdpAuthProvider, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(keySecretB64))
	if err != nil {
		keyBytes, err = base64.URLEncoding.DecodeString(strings.TrimSpace(keySecretB64))
		if err != nil {
			return nil, fmt.Errorf("invalid base64 key: %w", err)
		}
	}

	var privateKey interface{}
	var algorithm string

	switch {
	case len(keyBytes) == ed25519.PrivateKeySize: // 64-byte raw Ed25519
		privateKey = ed25519.PrivateKey(keyBytes)
		algorithm = "EdDSA"
	case len(keyBytes) == ed25519.SeedSize: // 32-byte Ed25519 seed
		privateKey = ed25519.NewKeyFromSeed(keyBytes)
		algorithm = "EdDSA"
	default:
		// Try PKCS8
		if key, pkErr := x509.ParsePKCS8PrivateKey(keyBytes); pkErr == nil {
			switch k := key.(type) {
			case *ecdsa.PrivateKey:
				privateKey = k
				algorithm = "ES256"
			case ed25519.PrivateKey:
				privateKey = k
				algorithm = "EdDSA"
			}
		}
		// Try SEC1/EC
		if privateKey == nil {
			if key, ecErr := x509.ParseECPrivateKey(keyBytes); ecErr == nil {
				privateKey = key
				algorithm = "ES256"
			}
		}
	}

	if privateKey == nil {
		return nil, fmt.Errorf("unsupported key format (tried Ed25519, PKCS8, SEC1)")
	}

	u, err := url.Parse(facilitatorURL)
	if err != nil {
		return nil, fmt.Errorf("invalid facilitator URL: %w", err)
	}

	return &cdpAuthProvider{
		keyID:       keyID,
		privateKey:  privateKey,
		algorithm:   algorithm,
		facHost:     u.Host,
		facBasePath: strings.TrimRight(u.Path, "/"),
	}, nil
}

// GetAuthHeaders generates fresh CDP JWT tokens for each facilitator endpoint.
func (a *cdpAuthProvider) GetAuthHeaders(_ context.Context) (fachttp.AuthHeaders, error) {
	verifyToken, err := a.generateJWT("POST", a.facBasePath+"/verify")
	if err != nil {
		return fachttp.AuthHeaders{}, fmt.Errorf("verify JWT: %w", err)
	}
	settleToken, err := a.generateJWT("POST", a.facBasePath+"/settle")
	if err != nil {
		return fachttp.AuthHeaders{}, fmt.Errorf("settle JWT: %w", err)
	}
	supportedToken, err := a.generateJWT("GET", a.facBasePath+"/supported")
	if err != nil {
		return fachttp.AuthHeaders{}, fmt.Errorf("supported JWT: %w", err)
	}

	return fachttp.AuthHeaders{
		Verify:    map[string]string{"Authorization": "Bearer " + verifyToken},
		Settle:    map[string]string{"Authorization": "Bearer " + settleToken},
		Supported: map[string]string{"Authorization": "Bearer " + supportedToken},
	}, nil
}

// generateJWT creates a short-lived CDP JWT for the given HTTP method and path.
// Format matches Coinbase Developer Platform API auth specification.
func (a *cdpAuthProvider) generateJWT(method, path string) (string, error) {
	now := time.Now()

	// Nonce: 8 random bytes as hex (16 chars)
	nonceBytes := make([]byte, 8)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}
	nonce := fmt.Sprintf("%x", nonceBytes)

	header := map[string]interface{}{
		"alg":   a.algorithm,
		"typ":   "JWT",
		"kid":   a.keyID,
		"nonce": nonce,
	}

	uri := method + " " + a.facHost + path
	claims := map[string]interface{}{
		"sub":  a.keyID,
		"iss":  "cdp",
		"aud":  []string{"cdp_service"},
		"nbf":  now.Unix(),
		"exp":  now.Add(120 * time.Second).Unix(),
		"uris": []string{uri},
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64

	var sigBytes []byte
	switch key := a.privateKey.(type) {
	case *ecdsa.PrivateKey:
		hash := sha256.Sum256([]byte(signingInput))
		r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
		if err != nil {
			return "", err
		}
		byteLen := (key.Curve.Params().BitSize + 7) / 8
		sigBytes = make([]byte, 2*byteLen)
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		copy(sigBytes[byteLen-len(rBytes):byteLen], rBytes)
		copy(sigBytes[2*byteLen-len(sBytes):2*byteLen], sBytes)
	case ed25519.PrivateKey:
		sigBytes = ed25519.Sign(key, []byte(signingInput))
	default:
		return "", fmt.Errorf("unsupported key type: %T", key)
	}

	sigB64 := base64.RawURLEncoding.EncodeToString(sigBytes)
	return signingInput + "." + sigB64, nil
}
