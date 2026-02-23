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
		Network:        "eip155:84532",
	}

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
		Network:        "eip155:84532",
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

	// Verify V2 JSON format
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("402 body is not valid JSON: %v", err)
	}
	if body["x402Version"] != float64(2) {
		t.Errorf("x402Version: want 2, got %v", body["x402Version"])
	}
}

func TestPricedMiddleware_ValidPayment_PassesThrough(t *testing.T) {
	fac := mockFacilitator(t, true)
	defer fac.Close()

	cfg := x402.MiddlewareConfig{
		WalletAddress:  "0xWALLET",
		FacilitatorURL: fac.URL,
		Network:        "eip155:84532",
	}

	mw := x402.NewPricedMiddleware(cfg, "0.001")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"ok"}`)) //nolint:errcheck
	}))

	// Simulate a V1 payment (raw JSON in X-Payment header)
	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/selic", nil)
	req.Header.Set("X-Payment", `{"x402Version":1,"scheme":"exact","payload":{"sig":"0xtest"}}`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 pass-through, got %d; body: %s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("X-PAYMENT-RESPONSE") == "" {
		t.Error("expected X-PAYMENT-RESPONSE header after settlement")
	}
}

