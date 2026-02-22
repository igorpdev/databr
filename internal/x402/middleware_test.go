package x402_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/x402"
)

// mockFacilitator returns a test server that simulates the x402 facilitator.
// isValid controls whether /verify returns {isValid:true} or {isValid:false}.
func mockFacilitator(t *testing.T, isValid bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/supported":
			// x402-go middleware calls /supported at startup to fetch chain config
			json.NewEncoder(w).Encode(map[string]any{
				"kinds": []map[string]any{
					{
						"scheme":  "exact",
						"network": "eip155:84532",
					},
				},
			})
		case "/verify":
			if isValid {
				json.NewEncoder(w).Encode(map[string]any{
					"isValid": true,
					"payer":   "0xCLIENT",
				})
			} else {
				json.NewEncoder(w).Encode(map[string]any{
					"isValid":       false,
					"invalidReason": "invalid_signature",
				})
			}
		case "/settle":
			json.NewEncoder(w).Encode(map[string]any{
				"success":     true,
				"transaction": "0xabc123",
				"network":     "eip155:84532",
				"payer":       "0xCLIENT",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestNewPricedMiddleware_Config(t *testing.T) {
	fac := mockFacilitator(t, true)
	defer fac.Close()

	cfg := x402.MiddlewareConfig{
		WalletAddress:  "0xWALLET",
		FacilitatorURL: fac.URL,
		Network:        "base-sepolia",
	}

	// NewPricedMiddleware should not panic even if route is unknown
	mw := x402.NewPricedMiddleware(cfg, "0.001")
	if mw == nil {
		t.Fatal("NewPricedMiddleware returned nil")
	}
}

func TestPricedMiddleware_NoPayment_Returns402(t *testing.T) {
	fac := mockFacilitator(t, true)
	defer fac.Close()

	cfg := x402.MiddlewareConfig{
		WalletAddress:  "0xWALLET",
		FacilitatorURL: fac.URL,
		Network:        "base-sepolia",
	}

	mw := x402.NewPricedMiddleware(cfg, "0.001")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/selic", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 Payment Required, got %d", rec.Code)
	}
}

func TestPricedMiddleware_HealthBypass(t *testing.T) {
	fac := mockFacilitator(t, true)
	defer fac.Close()

	cfg := x402.MiddlewareConfig{
		WalletAddress:  "0xWALLET",
		FacilitatorURL: fac.URL,
		Network:        "base-sepolia",
	}

	// /health endpoint must not require payment
	mw := x402.HealthBypassMiddleware(x402.NewPricedMiddleware(cfg, "0.001"))
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("/health should bypass x402, got %d", rec.Code)
	}
}
