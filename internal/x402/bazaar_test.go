package x402_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// sample402Body simulates a real 402 response from the x402 middleware with
// at least one accepts entry so the BazaarMiddleware can inject fields into it.
const sample402Body = `{"x402Version":1,"accepts":[{"scheme":"exact","network":"base","asset":"0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913","maxAmountRequired":"1000","payTo":"0xABC"}]}`

// TestBazaarMiddleware_InjectsIntoAccepts verifies that a 402 response gets
// discovery fields injected into each accepts item.
func TestBazaarMiddleware_InjectsIntoAccepts(t *testing.T) {
	r := chi.NewRouter()
	r.Use(x402.BazaarMiddleware())
	r.Get("/v1/bcb/selic", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(sample402Body)) //nolint:errcheck
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

	accepts, ok := body["accepts"].([]interface{})
	if !ok || len(accepts) == 0 {
		t.Fatalf("expected non-empty accepts array, body: %s", rr.Body.String())
	}

	item := accepts[0].(map[string]interface{})

	if item["discoverable"] != true {
		t.Errorf("discoverable: want true, got %v", item["discoverable"])
	}
	if item["method"] != "GET" {
		t.Errorf("method: want GET, got %v", item["method"])
	}
	if item["description"] != "Taxa Selic do Banco Central" {
		t.Errorf("description: want 'Taxa Selic do Banco Central', got %v", item["description"])
	}
	if item["mimeType"] != "application/json" {
		t.Errorf("mimeType: want application/json, got %v", item["mimeType"])
	}

	// Original 402 fields must still be present.
	if body["x402Version"] == nil {
		t.Error("original x402Version field was lost after injection")
	}
	if item["scheme"] != "exact" {
		t.Error("original accepts fields were lost")
	}
}

// TestBazaarMiddleware_PassesThrough200 verifies that successful responses are
// not modified.
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

	// 200 body should be untouched
	if rr.Body.String() != `{"data":"ok"}` {
		t.Errorf("200 body was modified: %s", rr.Body.String())
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
		w.Write([]byte(sample402Body)) //nolint:errcheck
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

	accepts := body["accepts"].([]interface{})
	item := accepts[0].(map[string]interface{})

	if item["discoverable"] != true {
		t.Error("unknown route should still be discoverable")
	}
	// Fallback description
	if item["description"] != "DataBR — dados públicos brasileiros" {
		t.Errorf("expected fallback description, got %v", item["description"])
	}
}
