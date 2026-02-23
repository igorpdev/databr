package x402_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// TestV2Response_KnownRoute verifies that a 402 response for a known route
// includes V2 format with resource metadata and Bazaar discovery fields in accepts.
func TestV2Response_KnownRoute(t *testing.T) {
	fac := mockFacilitator(t, true)
	defer fac.Close()

	r := chi.NewRouter()
	r.Use(x402.NewPricedMiddleware(x402.MiddlewareConfig{
		WalletAddress:  "0xWALLET",
		FacilitatorURL: fac.URL,
		Network:        "eip155:84532",
	}, "0.001"))
	r.Get("/v1/bcb/selic", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/selic", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v\nbody: %s", err, rr.Body.String())
	}

	// V2 format: x402Version = 2
	if body["x402Version"] != float64(2) {
		t.Errorf("x402Version: want 2, got %v", body["x402Version"])
	}

	// Resource metadata (V2 top-level)
	resource, ok := body["resource"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected resource object, got %v", body["resource"])
	}
	if resource["description"] != "Taxa Selic do Banco Central" {
		t.Errorf("resource.description: want 'Taxa Selic do Banco Central', got %v", resource["description"])
	}
	if resource["mimeType"] != "application/json" {
		t.Errorf("resource.mimeType: want application/json, got %v", resource["mimeType"])
	}

	// Accepts array with payment requirements + discovery fields
	accepts, ok := body["accepts"].([]interface{})
	if !ok || len(accepts) == 0 {
		t.Fatalf("expected non-empty accepts array, body: %s", rr.Body.String())
	}
	item := accepts[0].(map[string]interface{})
	if item["scheme"] != "exact" {
		t.Errorf("accepts[0].scheme: want exact, got %v", item["scheme"])
	}
	if item["network"] != "eip155:84532" {
		t.Errorf("accepts[0].network: want eip155:84532, got %v", item["network"])
	}

	// Bazaar discovery fields inside accepts item (V1-style, read by indexer)
	if item["description"] != "Taxa Selic do Banco Central" {
		t.Errorf("accepts[0].description: want 'Taxa Selic do Banco Central', got %v", item["description"])
	}
	if item["mimeType"] != "application/json" {
		t.Errorf("accepts[0].mimeType: want application/json, got %v", item["mimeType"])
	}
	if item["discoverable"] != true {
		t.Errorf("accepts[0].discoverable: want true, got %v", item["discoverable"])
	}

	schema, ok := item["outputSchema"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected outputSchema in accepts[0], got %v", item["outputSchema"])
	}
	input, ok := schema["input"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected outputSchema.input, got %v", schema["input"])
	}
	if input["discoverable"] != true {
		t.Errorf("outputSchema.input.discoverable: want true, got %v", input["discoverable"])
	}
	if input["method"] != "GET" {
		t.Errorf("outputSchema.input.method: want GET, got %v", input["method"])
	}
}

// TestV2Response_UnknownRoute uses a fallback description for routes not in routeMeta.
func TestV2Response_UnknownRoute(t *testing.T) {
	fac := mockFacilitator(t, true)
	defer fac.Close()

	r := chi.NewRouter()
	r.Use(x402.NewPricedMiddleware(x402.MiddlewareConfig{
		WalletAddress:  "0xWALLET",
		FacilitatorURL: fac.URL,
		Network:        "eip155:84532",
	}, "0.001"))
	r.Get("/v1/unknown/route", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/unknown/route", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}

	resource := body["resource"].(map[string]interface{})
	if resource["description"] != "DataBR — dados públicos brasileiros" {
		t.Errorf("expected fallback description, got %v", resource["description"])
	}

	// Discovery fields should still be present in accepts
	accepts := body["accepts"].([]interface{})
	item := accepts[0].(map[string]interface{})
	if item["discoverable"] != true {
		t.Error("unknown route should still have discoverable=true in accepts")
	}
}

// TestRouteMeta_Coverage verifies that all pricing table routes have metadata.
func TestRouteMeta_Coverage(t *testing.T) {
	for _, pattern := range x402.AllRoutePatterns() {
		desc, _ := x402.RouteMeta(pattern)
		if desc == "DataBR — dados públicos brasileiros" {
			t.Errorf("route %q uses fallback description — add it to routeMeta", pattern)
		}
	}
}
