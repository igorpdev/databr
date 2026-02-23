package comex_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/comex"
)

var fakeComexResponse = []map[string]any{
	{
		"coNcmSecrom": "01",
		"noNcmPor":    "Animais vivos",
		"coUnid":      "KG",
		"coStat":      "US$",
		"metStatFob":  float64(1500000),
		"metKg":       float64(250000),
	},
	{
		"coNcmSecrom": "09",
		"noNcmPor":    "Cafe, cha, mate e especiarias",
		"coUnid":      "KG",
		"coStat":      "US$",
		"metStatFob":  float64(8500000),
		"metKg":       float64(1200000),
	},
}

func newComexServer(t *testing.T, resp any, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestComexStatCollector_Source(t *testing.T) {
	srv := newComexServer(t, fakeComexResponse, http.StatusOK)
	defer srv.Close()

	c := comex.NewComexStatCollector(srv.URL)
	if got := c.Source(); got != "comex_exportacoes" {
		t.Errorf("Source() = %q, want %q", got, "comex_exportacoes")
	}
}

func TestComexStatCollector_Schedule(t *testing.T) {
	srv := newComexServer(t, fakeComexResponse, http.StatusOK)
	defer srv.Close()

	c := comex.NewComexStatCollector(srv.URL)
	if got := c.Schedule(); got != "0 8 1 * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 8 1 * *")
	}
}

func TestComexStatCollector_Collect(t *testing.T) {
	srv := newComexServer(t, fakeComexResponse, http.StatusOK)
	defer srv.Close()

	c := comex.NewComexStatCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "comex_exportacoes" {
		t.Errorf("Source = %q, want comex_exportacoes", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["noNcmPor"]; !ok {
		t.Error("Data must contain 'noNcmPor' field")
	}
	if _, ok := r.Data["metStatFob"]; !ok {
		t.Error("Data must contain 'metStatFob' field")
	}
}

func TestComexStatCollector_EmptyResponse(t *testing.T) {
	srv := newComexServer(t, []map[string]any{}, http.StatusOK)
	defer srv.Close()

	c := comex.NewComexStatCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

func TestComexStatCollector_HTTPError(t *testing.T) {
	srv := newComexServer(t, nil, http.StatusInternalServerError)
	defer srv.Close()

	c := comex.NewComexStatCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestComexStatCollector_RecordKeyUnique(t *testing.T) {
	srv := newComexServer(t, fakeComexResponse, http.StatusOK)
	defer srv.Close()

	c := comex.NewComexStatCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	seen := make(map[string]bool)
	for _, r := range records {
		if seen[r.RecordKey] {
			t.Errorf("duplicate RecordKey: %q", r.RecordKey)
		}
		seen[r.RecordKey] = true
	}
}
