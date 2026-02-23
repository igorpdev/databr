package cache_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/cache"
)

// mockCacher is an in-memory implementation of cache.Cacher for testing.
type mockCacher struct {
	store    map[string][]byte
	ttls     map[string]time.Duration
	storedAt map[string]time.Time
}

func newMockCacher() *mockCacher {
	return &mockCacher{
		store:    make(map[string][]byte),
		ttls:     make(map[string]time.Duration),
		storedAt: make(map[string]time.Time),
	}
}

func (m *mockCacher) Get(_ context.Context, key string, dest any) error {
	b, ok := m.store[key]
	if !ok {
		return cache.ErrCacheMiss
	}
	return json.Unmarshal(b, dest)
}

func (m *mockCacher) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.store[key] = b
	m.ttls[key] = ttl
	m.storedAt[key] = time.Now()
	return nil
}

func (m *mockCacher) TTL(_ context.Context, key string) (time.Duration, error) {
	ttl, ok := m.ttls[key]
	if !ok {
		return 0, nil
	}
	return ttl, nil
}

func TestCacheMiddleware_Miss_Then_Hit(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"source": "test", "data": map[string]any{"value": 1}})
	})

	mc := newMockCacher()
	mw := cache.NewCacheMiddleware(mc, 1*time.Hour)
	wrapped := mw(handler)

	// First request — MISS
	req1 := httptest.NewRequest("GET", "/v1/test", nil)
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	if rec1.Header().Get("X-Cache") != "MISS" {
		t.Errorf("first request X-Cache = %q, want MISS", rec1.Header().Get("X-Cache"))
	}
	if rec1.Code != 200 {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}

	// Second request — HIT
	req2 := httptest.NewRequest("GET", "/v1/test", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Header().Get("X-Cache") != "HIT" {
		t.Errorf("second request X-Cache = %q, want HIT", rec2.Header().Get("X-Cache"))
	}

	var resp map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal cached response: %v", err)
	}
	if cached, ok := resp["cached"].(bool); !ok || !cached {
		t.Errorf("cached = %v, want true", resp["cached"])
	}
	if _, ok := resp["cache_age_seconds"]; !ok {
		t.Error("cache_age_seconds missing from cached response")
	}
}

func TestCacheMiddleware_SkipsPOST(t *testing.T) {
	calls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"n": calls})
	})

	mc := newMockCacher()
	mw := cache.NewCacheMiddleware(mc, 1*time.Hour)
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/v1/carteira/risco", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	req2 := httptest.NewRequest("POST", "/v1/carteira/risco", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if calls != 2 {
		t.Errorf("POST handler called %d times, want 2 (no caching)", calls)
	}
}

func TestCacheMiddleware_SkipsNon200(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	})

	mc := newMockCacher()
	mw := cache.NewCacheMiddleware(mc, 1*time.Hour)
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/v1/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if len(mc.store) != 0 {
		t.Error("non-200 response should not be cached")
	}
}

func TestCacheMiddleware_NilCacher(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	mw := cache.NewCacheMiddleware(nil, 1*time.Hour)
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/v1/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("nil cacher status = %d, want 200", rec.Code)
	}
}

func TestCacheMiddleware_SortedQueryParams(t *testing.T) {
	calls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"n": calls})
	})

	mc := newMockCacher()
	mw := cache.NewCacheMiddleware(mc, 1*time.Hour)
	wrapped := mw(handler)

	// First request with params in one order
	req1 := httptest.NewRequest("GET", "/v1/test?b=2&a=1", nil)
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	// Second request with params in reversed order — should hit cache
	req2 := httptest.NewRequest("GET", "/v1/test?a=1&b=2", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if calls != 1 {
		t.Errorf("handler called %d times, want 1 (sorted params should match)", calls)
	}
	if rec2.Header().Get("X-Cache") != "HIT" {
		t.Errorf("sorted params X-Cache = %q, want HIT", rec2.Header().Get("X-Cache"))
	}
}
