package anp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/anp"
	"github.com/databr/api/internal/testutil"
	"github.com/xuri/excelize/v2"
)

// createTestXLSX builds a minimal XLSX file with a header and two data rows.
func createTestXLSX() []byte {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	// Header row
	f.SetCellValue(sheet, "A1", "REGIAO")
	f.SetCellValue(sheet, "B1", "ESTADO")
	f.SetCellValue(sheet, "C1", "PRODUTO")
	f.SetCellValue(sheet, "D1", "DATA INICIAL")
	f.SetCellValue(sheet, "E1", "DATA FINAL")
	f.SetCellValue(sheet, "F1", "PRECO MEDIO REVENDA")

	// Data row 1
	f.SetCellValue(sheet, "A2", "BRASIL")
	f.SetCellValue(sheet, "B2", "NACIONAL")
	f.SetCellValue(sheet, "C2", "GASOLINA COMUM")
	f.SetCellValue(sheet, "D2", "2026-01-01")
	f.SetCellValue(sheet, "E2", "2026-01-31")
	f.SetCellValue(sheet, "F2", "6.15")

	// Data row 2
	f.SetCellValue(sheet, "A3", "BRASIL")
	f.SetCellValue(sheet, "B3", "NACIONAL")
	f.SetCellValue(sheet, "C3", "ETANOL")
	f.SetCellValue(sheet, "D3", "2026-01-01")
	f.SetCellValue(sheet, "E3", "2026-01-31")
	f.SetCellValue(sheet, "F3", "4.25")

	buf, _ := f.WriteToBuffer()
	return buf.Bytes()
}

// createEmptyXLSX builds an XLSX with only the header row (no data).
func createEmptyXLSX() []byte {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	f.SetCellValue(sheet, "A1", "REGIAO")
	f.SetCellValue(sheet, "B1", "ESTADO")
	f.SetCellValue(sheet, "C1", "PRODUTO")
	f.SetCellValue(sheet, "D1", "DATA INICIAL")
	f.SetCellValue(sheet, "E1", "DATA FINAL")
	f.SetCellValue(sheet, "F1", "PRECO MEDIO REVENDA")

	buf, _ := f.WriteToBuffer()
	return buf.Bytes()
}

func newXLSXServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
}

func TestCombustiveisCollector_Source(t *testing.T) {
	c := anp.NewCombustiveisCollector("http://unused")
	if got := c.Source(); got != "anp_combustiveis" {
		t.Errorf("Source() = %q, want %q", got, "anp_combustiveis")
	}
}

func TestCombustiveisCollector_Schedule(t *testing.T) {
	c := anp.NewCombustiveisCollector("http://unused")
	if got := c.Schedule(); got != "@weekly" {
		t.Errorf("Schedule() = %q, want %q", got, "@weekly")
	}
}

func TestCombustiveisCollector_Collect(t *testing.T) {
	srv := newXLSXServer(t, createTestXLSX())
	defer srv.Close()

	c := anp.NewCombustiveisCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Collect() returned %d records, want 2", len(records))
	}

	// Verify first record (GASOLINA COMUM)
	r := records[0]
	if r.Source != "anp_combustiveis" {
		t.Errorf("Source = %q, want %q", r.Source, "anp_combustiveis")
	}
	if r.RecordKey != "gasolina_comum_2026-01" {
		t.Errorf("RecordKey = %q, want %q", r.RecordKey, "gasolina_comum_2026-01")
	}

	// Verify required data fields are present.
	requiredFields := []string{"produto", "regiao", "data_inicial", "data_final", "preco_medio_revenda"}
	for _, field := range requiredFields {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data must contain %q field; got keys: %v", field, testutil.DataKeys(r.Data))
		}
	}

	// Verify data values.
	if got := r.Data["produto"]; got != "GASOLINA COMUM" {
		t.Errorf("produto = %q, want %q", got, "GASOLINA COMUM")
	}
	if got := r.Data["preco_medio_revenda"]; got != "6.15" {
		t.Errorf("preco_medio_revenda = %q, want %q", got, "6.15")
	}

	// Verify second record (ETANOL)
	r2 := records[1]
	if r2.RecordKey != "etanol_2026-01" {
		t.Errorf("RecordKey = %q, want %q", r2.RecordKey, "etanol_2026-01")
	}
}

func TestCombustiveisCollector_EmptyXLSX(t *testing.T) {
	srv := newXLSXServer(t, createEmptyXLSX())
	defer srv.Close()

	c := anp.NewCombustiveisCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() on empty XLSX should not error, got: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Collect() on empty XLSX returned %d records, want 0", len(records))
	}
}

func TestCombustiveisCollector_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := anp.NewCombustiveisCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("Collect() should return error on HTTP 500")
	}
}

func TestCombustiveisCollector_HTMLContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Not a spreadsheet</body></html>"))
	}))
	defer srv.Close()

	c := anp.NewCombustiveisCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("Collect() should return error when server returns HTML instead of XLSX")
	}
}

func TestCombustiveisCollector_BRDateFormat(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	f.SetCellValue(sheet, "A1", "REGIAO")
	f.SetCellValue(sheet, "B1", "ESTADO")
	f.SetCellValue(sheet, "C1", "PRODUTO")
	f.SetCellValue(sheet, "D1", "DATA INICIAL")
	f.SetCellValue(sheet, "E1", "DATA FINAL")
	f.SetCellValue(sheet, "F1", "PRECO MEDIO REVENDA")

	// Use BR date format DD/MM/YYYY
	f.SetCellValue(sheet, "A2", "BRASIL")
	f.SetCellValue(sheet, "B2", "NACIONAL")
	f.SetCellValue(sheet, "C2", "DIESEL")
	f.SetCellValue(sheet, "D2", "01/03/2026")
	f.SetCellValue(sheet, "E2", "31/03/2026")
	f.SetCellValue(sheet, "F2", "5.89")

	buf, _ := f.WriteToBuffer()
	srv := newXLSXServer(t, buf.Bytes())
	defer srv.Close()

	c := anp.NewCombustiveisCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("Collect() returned %d records, want 1", len(records))
	}

	if records[0].RecordKey != "diesel_2026-03" {
		t.Errorf("RecordKey = %q, want %q", records[0].RecordKey, "diesel_2026-03")
	}
}

func TestCombustiveisCollector_DefaultURL(t *testing.T) {
	// Passing empty string should use the default URL, not panic.
	c := anp.NewCombustiveisCollector("")
	if c.Source() != "anp_combustiveis" {
		t.Errorf("Source() = %q after default URL init", c.Source())
	}
}

