package cvm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/cvm"
)

const fakeCotasCSV = `CNPJ_FUNDO;DT_COMPTC;VL_TOTAL;VL_QUOTA;VL_PATRIM_LIQ;CAPTC_DIA;RESG_DIA;NR_COTST
11.111.111/0001-11;2025-01-02;1000000.00;15.234567;1000000.00;0.00;0.00;100
22.222.222/0001-22;2025-01-02;2500000.00;25.678901;2400000.00;50000.00;10000.00;250`

func newCotasServer(t *testing.T, csvContent string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(csvContent))
	}))
}

func TestCotasCollector_Source(t *testing.T) {
	srv := newCotasServer(t, fakeCotasCSV)
	defer srv.Close()
	c := cvm.NewCotasCollector(srv.URL)
	if c.Source() != "cvm_cotas" {
		t.Errorf("Source() = %q, want cvm_cotas", c.Source())
	}
}

func TestCotasCollector_Schedule(t *testing.T) {
	srv := newCotasServer(t, fakeCotasCSV)
	defer srv.Close()
	c := cvm.NewCotasCollector(srv.URL)
	if c.Schedule() != "0 12 * * 1-5" {
		t.Errorf("Schedule() = %q, want 0 12 * * 1-5", c.Schedule())
	}
}

func TestCotasCollector_Collect(t *testing.T) {
	srv := newCotasServer(t, fakeCotasCSV)
	defer srv.Close()

	c := cvm.NewCotasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "cvm_cotas" {
		t.Errorf("Source = %q, want cvm_cotas", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	for _, field := range []string{"cnpj", "cnpj_digits", "data", "vl_quota", "vl_patrimonio", "captacao", "resgate", "nr_cotistas"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestCotasCollector_RecordKey(t *testing.T) {
	srv := newCotasServer(t, fakeCotasCSV)
	defer srv.Close()

	c := cvm.NewCotasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// RecordKey should be {cnpj_digits}_{date}
	expected := "11111111000111_2025-01-02"
	if records[0].RecordKey != expected {
		t.Errorf("RecordKey = %q, want %q", records[0].RecordKey, expected)
	}
}

func TestCotasCollector_CNPJDigits(t *testing.T) {
	srv := newCotasServer(t, fakeCotasCSV)
	defer srv.Close()

	c := cvm.NewCotasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// cnpj_digits should contain only digits
	cnpjDigits, ok := records[0].Data["cnpj_digits"].(string)
	if !ok {
		t.Fatal("cnpj_digits should be a string")
	}
	for _, ch := range cnpjDigits {
		if ch < '0' || ch > '9' {
			t.Errorf("cnpj_digits contains non-digit character: %q", ch)
		}
	}
	if cnpjDigits != "11111111000111" {
		t.Errorf("cnpj_digits = %q, want 11111111000111", cnpjDigits)
	}
}

func TestCotasCollector_EmptyCSV(t *testing.T) {
	srv := newCotasServer(t, "CNPJ_FUNDO;DT_COMPTC;VL_TOTAL;VL_QUOTA;VL_PATRIM_LIQ;CAPTC_DIA;RESG_DIA;NR_COTST\n")
	defer srv.Close()

	c := cvm.NewCotasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for header-only CSV, got %d", len(records))
	}
}
