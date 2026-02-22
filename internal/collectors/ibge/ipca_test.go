package ibge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/ibge"
)

// IBGE SIDRA returns flat JSON array
var fakeIPCAResponse = []map[string]any{
	{
		"NC":   "1",
		"NN":   "Brasil",
		"MC":   "2",
		"MN":   "%",
		"V":    "0.16",
		"D1C":  "1",
		"D2C":  "202601",
		"D2N":  "janeiro 2026",
		"D3C":  "63",
		"D3N":  "IPCA",
	},
}

func newSIDRAServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestIPCACollector_Source(t *testing.T) {
	srv := newSIDRAServer(t, fakeIPCAResponse)
	defer srv.Close()
	c := ibge.NewIPCACollector(srv.URL)
	if c.Source() != "ibge_ipca" {
		t.Errorf("Source() = %q, want ibge_ipca", c.Source())
	}
}

func TestIPCACollector_Schedule(t *testing.T) {
	srv := newSIDRAServer(t, fakeIPCAResponse)
	defer srv.Close()
	c := ibge.NewIPCACollector(srv.URL)
	// IPCA is released monthly
	if c.Schedule() != "0 8 * * *" {
		t.Errorf("Schedule() = %q, want daily at 08:00", c.Schedule())
	}
}

func TestIPCACollector_Collect(t *testing.T) {
	srv := newSIDRAServer(t, fakeIPCAResponse)
	defer srv.Close()

	c := ibge.NewIPCACollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	r := records[0]
	if r.Source != "ibge_ipca" {
		t.Errorf("Source = %q, want ibge_ipca", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty (should be the period, e.g. '202601')")
	}
	for _, field := range []string{"periodo", "variacao_pct", "indicador"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing normalized field %q", field)
		}
	}
}
