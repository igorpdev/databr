package cvm_test

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/cvm"
)

// buildZipCSV creates an in-memory ZIP file containing a CSV.
func buildZipCSV(t *testing.T, filename, csvContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(filename)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	f.Write([]byte(csvContent))
	w.Close()
	return buf.Bytes()
}

const fakeFundosCSV = `CNPJ_FUNDO;DT_COMPTC;VL_TOTAL;VL_QUOTA;VL_PATRIM_LIQ;CAPTC_DIA;RESG_DIA;NR_COTST
12345678000195;2026-01-31;1000000.00;10.5432;950000.00;50000.00;0.00;100
98765432000100;2026-01-31;2500000.00;25.1234;2400000.00;0.00;10000.00;250`

func newCVMServer(t *testing.T, csvContent string) *httptest.Server {
	t.Helper()
	zipData := buildZipCSV(t, "inf_diario_fi_202601.csv", csvContent)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipData)
	}))
}

func TestFundosCollector_Source(t *testing.T) {
	srv := newCVMServer(t, fakeFundosCSV)
	defer srv.Close()
	c := cvm.NewFundosCollector(srv.URL)
	if c.Source() != "cvm_fundos" {
		t.Errorf("Source() = %q, want cvm_fundos", c.Source())
	}
}

func TestFundosCollector_Schedule(t *testing.T) {
	srv := newCVMServer(t, fakeFundosCSV)
	defer srv.Close()
	c := cvm.NewFundosCollector(srv.URL)
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestFundosCollector_Collect(t *testing.T) {
	srv := newCVMServer(t, fakeFundosCSV)
	defer srv.Close()

	c := cvm.NewFundosCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "cvm_fundos" {
		t.Errorf("Source = %q, want cvm_fundos", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	for _, field := range []string{"cnpj_fundo", "data_competencia", "vl_quota", "vl_patrim_liq"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestFundosCollector_EmptyCSV(t *testing.T) {
	// Only headers, no data rows
	srv := newCVMServer(t, "CNPJ_FUNDO;DT_COMPTC;VL_TOTAL;VL_QUOTA;VL_PATRIM_LIQ;CAPTC_DIA;RESG_DIA;NR_COTST\n")
	defer srv.Close()

	c := cvm.NewFundosCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for header-only CSV, got %d", len(records))
	}
}
