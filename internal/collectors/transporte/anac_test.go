package transporte_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/transporte"
)

// utf8BOM is the UTF-8 byte order mark prepended to the real ANAC CSV.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// minimalRABCSV is a small CSV with 3 aircraft rows:
//   - PR-ABC: valid row
//   - PT-XYZ: valid row (different UF)
//   - (empty MARCA): must be skipped
//
// Delimiter is semicolon; encoding is UTF-8 (BOM added separately in tests).
const minimalRABCSV = "MARCA;PROPRIETARIOS;NM_OPERADOR;OUTROS_OPERADORES;CPF_CNPJ;SG_UF;UF_OPERADOR;NR_CERT_MATRICULA;NR_SERIE;CD_TIPO;DS_MODELO;NM_FABRICANTE;NR_ANO_FABRICACAO;DT_VALIDADE_CVA;DT_VALIDADE_CA;DT_MATRICULA;TP_OPERACAO\r\n" +
	"PR-ABC;JOAO DA SILVA - 123.456.789-00 - 100%;EMPRESA AEREA LTDA;;123.456.789-00;SP;SP;001234;12345;HA;R44 II;ROBINSON;2010;2026-12-31;2027-01-01;2010-06-15;PRIVADA\r\n" +
	"PT-XYZ;MARIA SOUZA - 00.000.000/0001-00 - 100%;OUTRA EMPRESA SA;;00.000.000/0001-00;RJ;RJ;005678;67890;HA;172C;CESSNA;2005;2025-06-30;2025-07-01;2005-03-20;PRIVADA\r\n" +
	";SEM MARCA;OPERADOR QUALQUER;;11.111.111/0001-11;MG;MG;009999;11111;HA;EMB-202;NEIVA;2000;2024-01-01;2024-02-01;2000-01-01;PRIVADA\r\n"

// newRABServer creates an httptest.Server that serves the given body bytes.
func newRABServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(body) //nolint:errcheck
	}))
}

func TestANACCollector_Source(t *testing.T) {
	body := append(utf8BOM, []byte(minimalRABCSV)...)
	srv := newRABServer(t, body)
	defer srv.Close()

	c := transporte.NewANACCollector(srv.URL)
	if got := c.Source(); got != "anac_rab" {
		t.Errorf("Source() = %q, want %q", got, "anac_rab")
	}
}

func TestANACCollector_Schedule(t *testing.T) {
	body := append(utf8BOM, []byte(minimalRABCSV)...)
	srv := newRABServer(t, body)
	defer srv.Close()

	c := transporte.NewANACCollector(srv.URL)
	if got := c.Schedule(); got != "@weekly" {
		t.Errorf("Schedule() = %q, want %q", got, "@weekly")
	}
}

func TestANACCollector_Collect_BOMStripped(t *testing.T) {
	// Serve CSV with BOM prefix — collector must handle it without error.
	body := append(utf8BOM, []byte(minimalRABCSV)...)
	srv := newRABServer(t, body)
	defer srv.Close()

	c := transporte.NewANACCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Expect exactly 2 records (empty-MARCA row is skipped).
	if len(records) != 2 {
		t.Fatalf("Collect() returned %d records, want 2", len(records))
	}
}

func TestANACCollector_Collect_RecordKey(t *testing.T) {
	body := append(utf8BOM, []byte(minimalRABCSV)...)
	srv := newRABServer(t, body)
	defer srv.Close()

	c := transporte.NewANACCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if records[0].RecordKey != "PR-ABC" {
		t.Errorf("records[0].RecordKey = %q, want %q", records[0].RecordKey, "PR-ABC")
	}
	if records[1].RecordKey != "PT-XYZ" {
		t.Errorf("records[1].RecordKey = %q, want %q", records[1].RecordKey, "PT-XYZ")
	}
}

func TestANACCollector_Collect_Source(t *testing.T) {
	body := append(utf8BOM, []byte(minimalRABCSV)...)
	srv := newRABServer(t, body)
	defer srv.Close()

	c := transporte.NewANACCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.Source != "anac_rab" {
			t.Errorf("record Source = %q, want %q", rec.Source, "anac_rab")
		}
	}
}

func TestANACCollector_Collect_DataFields(t *testing.T) {
	body := append(utf8BOM, []byte(minimalRABCSV)...)
	srv := newRABServer(t, body)
	defer srv.Close()

	c := transporte.NewANACCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one record")
	}

	rec := records[0]
	requiredFields := []string{
		"marca", "proprietarios", "operador", "uf", "modelo", "fabricante",
		"ano_fabricacao", "data_matricula", "tp_operacao",
	}
	for _, field := range requiredFields {
		if _, ok := rec.Data[field]; !ok {
			t.Errorf("Data must contain field %q; got keys: %v", field, dataKeys(rec.Data))
		}
	}

	// Spot-check specific values.
	if got, _ := rec.Data["fabricante"].(string); got != "ROBINSON" {
		t.Errorf("fabricante = %q, want ROBINSON", got)
	}
	if got, _ := rec.Data["uf"].(string); got != "SP" {
		t.Errorf("uf = %q, want SP", got)
	}
}

func TestANACCollector_Collect_SkipsEmptyMarca(t *testing.T) {
	body := append(utf8BOM, []byte(minimalRABCSV)...)
	srv := newRABServer(t, body)
	defer srv.Close()

	c := transporte.NewANACCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.RecordKey == "" {
			t.Error("found record with empty RecordKey (MARCA); should have been skipped")
		}
	}
}

func TestANACCollector_Collect_NoBOM(t *testing.T) {
	// Serve same CSV without BOM — collector must handle it too.
	body := []byte(minimalRABCSV)
	srv := newRABServer(t, body)
	defer srv.Close()

	c := transporte.NewANACCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("Collect() returned %d records, want 2", len(records))
	}
}

func dataKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
