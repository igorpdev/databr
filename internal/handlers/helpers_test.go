package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
