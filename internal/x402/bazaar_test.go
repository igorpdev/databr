package x402_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// TestBazaarMiddleware_Injects402Extension verifies that a 402 response gets
// the extensions.bazaar field injected with correct metadata for the matched route.
func TestBazaarMiddleware_Injects402Extension(t *testing.T) {
	r := chi.NewRouter()
	r.Use(x402.BazaarMiddleware())
	r.Get("/v1/bcb/selic", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(`{"x402Version":1,"accepts":[]}`)) //nolint:errcheck
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

	ext, ok := body["extensions"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected extensions field, body: %s", rr.Body.String())
	}

	bazaar, ok := ext["bazaar"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected extensions.bazaar field, extensions: %v", ext)
	}

	if bazaar["discoverable"] != true {
		t.Errorf("discoverable: want true, got %v", bazaar["discoverable"])
	}
	if bazaar["method"] != "GET" {
		t.Errorf("method: want GET, got %v", bazaar["method"])
	}
	if bazaar["description"] != "Taxa Selic do Banco Central" {
		t.Errorf("description: want 'Taxa Selic do Banco Central', got %v", bazaar["description"])
	}
	if bazaar["outputMimeType"] != "application/json" {
		t.Errorf("outputMimeType: want application/json, got %v", bazaar["outputMimeType"])
	}

	// Original 402 fields must still be present.
	if body["x402Version"] == nil {
		t.Error("original x402Version field was lost after injection")
	}
}

// TestBazaarMiddleware_PassesThrough200 verifies that successful responses are
// not modified and do not receive the bazaar extension.
func TestBazaarMiddleware_PassesThrough200(t *testing.T) {
	r := chi.NewRouter()
	r.Use(x402.BazaarMiddleware())
	r.Get("/v1/bcb/selic", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":"ok"}`)) //nolint:errcheck
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/bcb/selic", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}

	if _, hasExt := body["extensions"]; hasExt {
		t.Error("200 response must not contain extensions field")
	}
}

// TestBazaarMiddleware_UnknownRoute uses a fallback description when the route
// pattern is not in the routeMeta table.
func TestBazaarMiddleware_UnknownRoute(t *testing.T) {
	r := chi.NewRouter()
	r.Use(x402.BazaarMiddleware())
	r.Get("/v1/unknown/route", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(`{"x402Version":1,"accepts":[]}`)) //nolint:errcheck
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

	ext, ok := body["extensions"].(map[string]interface{})
	if !ok {
		t.Fatal("unknown route should still get extensions field")
	}
	bazaar := ext["bazaar"].(map[string]interface{})
	if bazaar["discoverable"] != true {
		t.Error("unknown route should still be discoverable")
	}
}
