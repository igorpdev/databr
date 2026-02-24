package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"time"

	"github.com/databr/api/internal/metrics"
)

// Cacher is the interface the cache middleware needs.
// Matches *Client methods but allows test mocks.
type Cacher interface {
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
}

// cachedResponse is what gets stored in Redis.
type cachedResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       json.RawMessage   `json:"body"`
	StoredAt   time.Time         `json:"stored_at"`
}

// NewCacheMiddleware returns a Chi middleware that caches GET 200 responses in Redis.
// POST/PUT/DELETE and non-200 responses are never cached.
// If cacher is nil, the middleware is a pass-through.
func NewCacheMiddleware(cacher Cacher, ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cacher == nil || r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			key := cacheKey(r)
			ctx := r.Context()

			// Try cache hit
			var cached cachedResponse
			if err := cacher.Get(ctx, key, &cached); err == nil {
				metrics.CacheHits.WithLabelValues(r.URL.Path, "true").Inc()
				age := int(time.Since(cached.StoredAt).Seconds())
				body := injectCacheFields(cached.Body, age)
				for k, v := range cached.Headers {
					w.Header().Set(k, v)
				}
				w.Header().Set("X-Cache", "HIT")
				w.Header().Set("Cache-Control", cacheControlHeader(ttl))
				w.WriteHeader(cached.StatusCode)
				w.Write(body)
				return
			}

			// Cache miss — capture response via recorder
			metrics.CacheHits.WithLabelValues(r.URL.Path, "false").Inc()
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)

			// Copy captured response to the real writer
			for k, vals := range rec.Header() {
				for _, v := range vals {
					w.Header().Set(k, v)
				}
			}
			w.Header().Set("X-Cache", "MISS")
			w.Header().Set("Cache-Control", cacheControlHeader(ttl))
			w.WriteHeader(rec.Code)
			w.Write(rec.Body.Bytes())

			// Only cache 200 OK
			if rec.Code == http.StatusOK {
				headers := make(map[string]string, len(rec.Header()))
				for k := range rec.Header() {
					headers[k] = rec.Header().Get(k)
				}
				entry := cachedResponse{
					StatusCode: rec.Code,
					Headers:    headers,
					Body:       rec.Body.Bytes(),
					StoredAt:   time.Now(),
				}
				if err := cacher.Set(ctx, key, entry, ttl); err != nil {
					slog.Warn("cache set failed", "key", key, "error", err)
				}
			}
		})
	}
}

// cacheKey builds a deterministic key from method + path + sorted query params.
func cacheKey(r *http.Request) string {
	var b strings.Builder
	b.WriteString("cache:")
	b.WriteString(r.Method)
	b.WriteString(":")
	b.WriteString(r.URL.Path)

	params := r.URL.Query()
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("?")
		for i, k := range keys {
			if i > 0 {
				b.WriteString("&")
			}
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(params.Get(k))
		}
	}
	return b.String()
}

// injectCacheFields sets "cached":true and "cache_age_seconds":N in a JSON body.
func injectCacheFields(body []byte, ageSeconds int) []byte {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	m["cached"] = true
	m["cache_age_seconds"] = ageSeconds
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}

// cacheControlHeader returns the Cache-Control value for the given TTL.
func cacheControlHeader(ttl time.Duration) string {
	secs := int(ttl.Seconds())
	if secs <= 0 {
		return "no-store"
	}
	return fmt.Sprintf("public, max-age=%d", secs)
}
