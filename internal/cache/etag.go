package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
)

// ETagMiddleware computes an ETag from the response body and returns
// 304 Not Modified when the client sends a matching If-None-Match header.
// Place this BEFORE compression middleware so the hash is computed on the
// uncompressed body (consistent regardless of Accept-Encoding).
func ETagMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to GET requests
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		// Capture the response
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)

		// Compute ETag from body (truncated SHA-256)
		body := rec.Body.Bytes()
		hash := sha256.Sum256(body)
		etag := `"` + hex.EncodeToString(hash[:8]) + `"`

		// Check If-None-Match
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// Copy response to the real writer
		for k, vals := range rec.Header() {
			for _, v := range vals {
				w.Header().Set(k, v)
			}
		}
		w.Header().Set("ETag", etag)
		w.WriteHeader(rec.Code)
		w.Write(body)
	})
}
