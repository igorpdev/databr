package x402_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/x402"
)

func TestWellKnownHandler(t *testing.T) {
	cfg := x402.MiddlewareConfig{
		WalletAddress: "0xTESTWALLET",
		Network:       "eip155:8453",
	}

	handler := x402.WellKnownHandler(cfg)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/x402", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var resp x402.WellKnownResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.X402Version != 2 {
		t.Errorf("expected x402Version 2, got %d", resp.X402Version)
	}
	if resp.PayTo != "0xTESTWALLET" {
		t.Errorf("expected payTo 0xTESTWALLET, got %s", resp.PayTo)
	}
	if resp.Network != "eip155:8453" {
		t.Errorf("expected network eip155:8453, got %s", resp.Network)
	}
	if resp.Asset == "" {
		t.Error("asset must not be empty")
	}
	if len(resp.Endpoints) == 0 {
		t.Error("endpoints must not be empty")
	}

	// x402scan discovery format checks.
	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
	if len(resp.Resources) == 0 {
		t.Error("resources must not be empty")
	}
	if len(resp.Resources) != len(resp.Endpoints) {
		t.Errorf("resources len %d != endpoints len %d", len(resp.Resources), len(resp.Endpoints))
	}
	for _, r := range resp.Resources {
		if !strings.HasPrefix(r, "https://") {
			t.Errorf("resource %q is not an absolute HTTPS URL", r)
		}
		if strings.Contains(r, "{") {
			t.Errorf("resource %q still contains unresolved chi parameter", r)
		}
	}

	// Every endpoint must have a non-empty path, description, and amount.
	for _, ep := range resp.Endpoints {
		if ep.Path == "" {
			t.Error("endpoint has empty path")
		}
		if ep.Amount == "" || ep.Amount == "0" {
			t.Errorf("endpoint %s has invalid amount %q", ep.Path, ep.Amount)
		}
		if ep.PriceUSDC == "" {
			t.Errorf("endpoint %s has empty priceUSDC", ep.Path)
		}
	}
}
