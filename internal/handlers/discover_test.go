package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverHandler_GetCases(t *testing.T) {
	h := NewDiscoverHandler()
	req := httptest.NewRequest("GET", "/v1/discover/cases", nil)
	w := httptest.NewRecorder()
	h.GetCases(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var envelope struct {
		Source string `json:"source"`
		Data   struct {
			Cases []struct {
				ID           string   `json:"id"`
				Title        string   `json:"title"`
				Endpoints    []string `json:"endpoints_used"`
				CostEstimate string   `json:"estimated_cost_usdc"`
			} `json:"cases"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Source != "databr_discover" {
		t.Errorf("source = %q, want databr_discover", envelope.Source)
	}
	if len(envelope.Data.Cases) < 5 {
		t.Fatalf("expected >= 5 cases, got %d", len(envelope.Data.Cases))
	}
	for _, c := range envelope.Data.Cases {
		if c.ID == "" || c.Title == "" || len(c.Endpoints) == 0 || c.CostEstimate == "" {
			t.Errorf("case %q missing required fields", c.ID)
		}
	}
}
