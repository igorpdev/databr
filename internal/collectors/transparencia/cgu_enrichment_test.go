package transparencia_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/collectors/transparencia"
)

func TestCGUCollector_FetchByCNPJ_EnrichedFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/ceis"):
			w.Write([]byte(`[]`))
		case strings.Contains(r.URL.Path, "/cnep"):
			w.Write([]byte(`[]`))
		case strings.Contains(r.URL.Path, "/pgfn"):
			w.Write([]byte(`[{"situacao":"REGULAR"}]`))
		case strings.Contains(r.URL.Path, "/leniencias"):
			w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := transparencia.NewCGUCollector(srv.URL, "test-key")
	records, err := c.FetchByCNPJ(context.Background(), "12345678000195")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	data := records[0].Data
	if _, ok := data["pgfn"]; !ok {
		t.Error("expected 'pgfn' key in data")
	}
	if _, ok := data["leniencias"]; !ok {
		t.Error("expected 'leniencias' key in data")
	}
	// pgfn has 1 item → not sanitized
	if san, _ := data["sanitized"].(bool); san {
		t.Error("expected sanitized=false when pgfn has entries")
	}
}

func TestCGUCollector_FetchByCNPJ_SanitizedWhenAllEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := transparencia.NewCGUCollector(srv.URL, "test-key")
	records, err := c.FetchByCNPJ(context.Background(), "12345678000195")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := records[0].Data
	if san, _ := data["sanitized"].(bool); !san {
		t.Error("expected sanitized=true when all lists empty")
	}
}

func TestCGUCollector_FetchByCNPJ_PGFNSoftFail(t *testing.T) {
	// PGFN returns 500 — should soft-fail (not return error, pgfn = [])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/pgfn") {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := transparencia.NewCGUCollector(srv.URL, "test-key")
	records, err := c.FetchByCNPJ(context.Background(), "12345678000195")
	// should NOT error even though PGFN failed
	if err != nil {
		t.Fatalf("unexpected error on PGFN soft-fail: %v", err)
	}
	data := records[0].Data
	pgfn, ok := data["pgfn"].([]any)
	if !ok {
		t.Fatal("expected pgfn to be []any")
	}
	if len(pgfn) != 0 {
		t.Errorf("expected empty pgfn on failure, got %v", pgfn)
	}
}
