package cache

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestETagMiddleware_SetsETag(t *testing.T) {
	handler := ETagMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hello":"world"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header to be set")
	}
	if rec.Body.String() != `{"hello":"world"}` {
		t.Fatalf("body mismatch: %s", rec.Body.String())
	}
}

func TestETagMiddleware_304OnMatch(t *testing.T) {
	handler := ETagMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hello":"world"}`))
	}))

	// First request to get the ETag
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")

	// Second request with If-None-Match
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("expected empty body on 304, got %d bytes", rec2.Body.Len())
	}
}

func TestETagMiddleware_SkipsNonGET(t *testing.T) {
	handler := ETagMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`ok`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("ETag") != "" {
		t.Fatal("ETag should not be set for POST requests")
	}
}

func TestETagMiddleware_DifferentBodiesGetDifferentETags(t *testing.T) {
	makeHandler := func(body string) http.Handler {
		return ETagMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(body))
		}))
	}

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	makeHandler("body-a").ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	makeHandler("body-b").ServeHTTP(rec2, req2)

	if rec1.Header().Get("ETag") == rec2.Header().Get("ETag") {
		t.Fatal("different bodies should produce different ETags")
	}
}
