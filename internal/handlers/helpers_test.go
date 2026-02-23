package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
)

func TestJsonError(t *testing.T) {
	w := httptest.NewRecorder()
	jsonError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["error"] != "test error" {
		t.Errorf("error = %q, want %q", body["error"], "test error")
	}
}

func TestGatewayError(t *testing.T) {
	w := httptest.NewRecorder()
	gatewayError(w, "test_source", http.ErrServerClosed)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Should NOT expose the actual error
	if body["error"] != "upstream service temporarily unavailable" {
		t.Errorf("error = %q, want generic message", body["error"])
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	internalError(w, "test_source", http.ErrServerClosed)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["error"] != "internal error" {
		t.Errorf("error = %q, want 'internal error'", body["error"])
	}
}

func TestRateLimitExceeded(t *testing.T) {
	w := httptest.NewRecorder()
	RateLimitExceeded(w)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"", defaultPageSize, 0},
		{"limit=10", 10, 0},
		{"limit=10&offset=20", 10, 20},
		{"limit=1000", maxPageSize, 0}, // clamped to max
		{"limit=-5", defaultPageSize, 0},
		{"limit=abc", defaultPageSize, 0},
		{"offset=-1", defaultPageSize, 0},
	}
	for _, tt := range tests {
		r := httptest.NewRequest("GET", "/?"+tt.query, nil)
		limit, offset := parsePagination(r)
		if limit != tt.wantLimit || offset != tt.wantOffset {
			t.Errorf("parsePagination(%q) = (%d, %d), want (%d, %d)",
				tt.query, limit, offset, tt.wantLimit, tt.wantOffset)
		}
	}
}

func TestPaginateSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Normal pagination
	got := paginateSlice(items, 3, 0)
	if len(got) != 3 || got[0] != 1 {
		t.Errorf("paginateSlice(3,0) = %v, want [1,2,3]", got)
	}

	// Offset
	got = paginateSlice(items, 3, 5)
	if len(got) != 3 || got[0] != 6 {
		t.Errorf("paginateSlice(3,5) = %v, want [6,7,8]", got)
	}

	// Beyond end
	got = paginateSlice(items, 5, 8)
	if len(got) != 2 {
		t.Errorf("paginateSlice(5,8) = %v, want 2 items", got)
	}

	// Offset past end
	got = paginateSlice(items, 5, 15)
	if got != nil {
		t.Errorf("paginateSlice(5,15) = %v, want nil", got)
	}
}

func TestProjectFields(t *testing.T) {
	data := map[string]any{
		"source": "bcb_selic",
		"valor":  "13.75",
		"data":   "2026-02-20",
		"total":  42,
	}

	// Select specific fields
	got := projectFields(data, "source,total")
	if len(got) != 2 {
		t.Fatalf("projectFields returned %d fields, want 2", len(got))
	}
	if got["source"] != "bcb_selic" {
		t.Errorf("source = %v, want bcb_selic", got["source"])
	}
	if got["total"] != 42 {
		t.Errorf("total = %v, want 42", got["total"])
	}

	// Unknown fields → empty map
	got = projectFields(data, "nonexistent")
	if len(got) != 0 {
		t.Errorf("projectFields with unknown fields returned %d entries, want 0", len(got))
	}

	// Spaces around field names
	got = projectFields(data, " valor , data ")
	if len(got) != 2 {
		t.Errorf("projectFields with spaces returned %d fields, want 2", len(got))
	}
}

func TestRespondWithFields(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/?fields=source,total", nil)
	r = x402pkg.InjectPrice(r, "0.003")

	respond(w, r, domain.APIResponse{
		Source:   "test",
		CostUSDC: "0.003",
		Data: map[string]any{
			"source": "bcb_selic",
			"valor":  "13.75",
			"total":  42,
		},
	})

	var resp domain.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Errorf("Data has %d fields, want 2 (source, total)", len(resp.Data))
	}
	if _, ok := resp.Data["valor"]; ok {
		t.Error("Data should not contain 'valor' after field projection")
	}
}

func TestParseDateFilter(t *testing.T) {
	// Valid date
	r := httptest.NewRequest("GET", "/?since=2026-01-15", nil)
	got := parseDateFilter(r, "since")
	if got == nil {
		t.Fatal("parseDateFilter returned nil for valid date")
	}
	want := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseDateFilter = %v, want %v", got, want)
	}

	// Missing param
	r = httptest.NewRequest("GET", "/", nil)
	if parseDateFilter(r, "since") != nil {
		t.Error("parseDateFilter should return nil for missing param")
	}

	// Invalid format
	r = httptest.NewRequest("GET", "/?since=not-a-date", nil)
	if parseDateFilter(r, "since") != nil {
		t.Error("parseDateFilter should return nil for invalid date")
	}
}

func TestRespondWithSinceFilter(t *testing.T) {
	// UpdatedAt is 2026-01-01, since=2026-02-01 → should get empty note
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/?since=2026-02-01", nil)
	r = x402pkg.InjectPrice(r, "0.003")

	respond(w, r, domain.APIResponse{
		Source:    "test",
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CostUSDC:  "0.003",
		Data:      map[string]any{"valor": "123"},
	})

	var resp domain.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Data["note"] != "no data in requested time range" {
		t.Errorf("expected time range note, got data: %v", resp.Data)
	}
}

func TestRespondWithUntilFilter(t *testing.T) {
	// UpdatedAt is 2026-03-01, until=2026-02-01 → should get empty note
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/?until=2026-02-01", nil)
	r = x402pkg.InjectPrice(r, "0.003")

	respond(w, r, domain.APIResponse{
		Source:    "test",
		UpdatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		CostUSDC:  "0.003",
		Data:      map[string]any{"valor": "123"},
	})

	var resp domain.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Data["note"] != "no data in requested time range" {
		t.Errorf("expected time range note, got data: %v", resp.Data)
	}
}

func TestRespondFieldsAndSinceCombined(t *testing.T) {
	// UpdatedAt is 2026-02-15, since=2026-01-01 → data passes filter
	// fields=valor → only valor field returned
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/?since=2026-01-01&fields=valor", nil)
	r = x402pkg.InjectPrice(r, "0.003")

	respond(w, r, domain.APIResponse{
		Source:    "test",
		UpdatedAt: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		CostUSDC:  "0.003",
		Data:      map[string]any{"valor": "123", "extra": "456"},
	})

	var resp domain.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Errorf("Data has %d fields, want 1", len(resp.Data))
	}
	if resp.Data["valor"] != "123" {
		t.Errorf("valor = %v, want 123", resp.Data["valor"])
	}
}
