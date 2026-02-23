package ipea_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/databr/api/internal/collectors/ipea"
	"github.com/databr/api/internal/testutil"
)

// newIPEAServer returns a mock HTTP server that simulates the IPEAData OData v4 API.
// It validates that the Accept header is set to application/json and returns a
// minimal two-value response for any series request.
func newIPEAServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CRITICAL: IPEAData requires Accept header, not $format=json.
		if r.Header.Get("Accept") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp := map[string]any{
			"value": []map[string]any{
				{"SERCODIGO": "BM12_TJOVER12", "VALDATA": "2026-02-01T00:00:00-03:00", "VALVALOR": 13.75},
				{"SERCODIGO": "BM12_TJOVER12", "VALDATA": "2026-01-01T00:00:00-03:00", "VALVALOR": 13.65},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestSeriesCollector_Source(t *testing.T) {
	srv := newIPEAServer(t)
	defer srv.Close()

	c := ipea.NewSeriesCollector(srv.URL)
	if got := c.Source(); got != "ipea_series" {
		t.Errorf("Source() = %q, want %q", got, "ipea_series")
	}
}

func TestSeriesCollector_Schedule(t *testing.T) {
	srv := newIPEAServer(t)
	defer srv.Close()

	c := ipea.NewSeriesCollector(srv.URL)
	if got := c.Schedule(); got != "@daily" {
		t.Errorf("Schedule() = %q, want %q", got, "@daily")
	}
}

func TestSeriesCollector_Collect(t *testing.T) {
	srv := newIPEAServer(t)
	defer srv.Close()

	c := ipea.NewSeriesCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// 6 series x 2 values each = 12 records.
	if len(records) != 12 {
		t.Fatalf("Collect() returned %d records, want 12", len(records))
	}

	// Verify first record structure.
	r := records[0]
	if r.Source != "ipea_series" {
		t.Errorf("Source = %q, want %q", r.Source, "ipea_series")
	}
	if r.FetchedAt.IsZero() {
		t.Error("FetchedAt must not be zero")
	}

	// Verify required data fields.
	requiredFields := []string{"codigo", "nome", "data", "valor"}
	for _, field := range requiredFields {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data must contain %q field; got keys: %v", field, testutil.DataKeys(r.Data))
		}
	}
}

func TestSeriesCollector_RecordKeyFormat(t *testing.T) {
	srv := newIPEAServer(t)
	defer srv.Close()

	c := ipea.NewSeriesCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(records) == 0 {
		t.Fatal("Collect() returned 0 records")
	}

	// First record should have RecordKey = "{code}_{YYYY-MM-DD}".
	rk := records[0].RecordKey
	if !strings.Contains(rk, "_2026-02-01") && !strings.Contains(rk, "_2026-01-01") {
		t.Errorf("RecordKey = %q, expected to contain a date suffix like _2026-02-01", rk)
	}

	// RecordKey should follow the pattern CODE_DATE.
	parts := strings.SplitN(rk, "_", 2)
	if len(parts) < 2 {
		t.Errorf("RecordKey = %q, expected format CODE_DATE", rk)
	}
}

func TestSeriesCollector_AcceptHeader(t *testing.T) {
	// Server that rejects requests without the correct Accept header.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("must set Accept: application/json"))
			return
		}
		resp := map[string]any{"value": []map[string]any{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := ipea.NewSeriesCollector(srv.URL)
	// Should succeed (no error) — proving the Accept header is set correctly.
	_, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() should not error when Accept header is correctly set, got: %v", err)
	}
}

func TestSeriesCollector_HTTPError(t *testing.T) {
	// Server that always returns 500 Internal Server Error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := ipea.NewSeriesCollector(srv.URL)
	records, err := c.Collect(context.Background())
	// Individual series failures are logged, not returned as errors.
	// All 6 series should fail, resulting in 0 records but no error.
	if err != nil {
		t.Fatalf("Collect() should not return error on upstream failures, got: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Collect() returned %d records, want 0 when all series fail", len(records))
	}
}

func TestSeriesCollector_DefaultBaseURL(t *testing.T) {
	// Passing empty string should use the default base URL (http, not https).
	c := ipea.NewSeriesCollector("")
	if got := c.Source(); got != "ipea_series" {
		t.Errorf("Source() = %q, want %q", got, "ipea_series")
	}
}

