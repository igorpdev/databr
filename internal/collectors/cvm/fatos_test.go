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

// buildFatosZip creates an in-memory ZIP file containing a CVM IPE CSV.
// The CSV uses semicolons as delimiters and is treated as ISO-8859-1.
func buildFatosZip(t *testing.T, csvContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("ipe_cia_aberta_2026.csv")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	f.Write([]byte(csvContent))
	w.Close()
	return buf.Bytes()
}

// fakeFatosCSV contains the real IPE CSV header plus 3 rows:
//   - 1 Fato Relevante (should be collected)
//   - 1 Comunicado ao Mercado (should be filtered out)
//   - 1 Fato Relevante with a different company (should be collected)
const fakeFatosCSV = "CNPJ_Companhia;Nome_Companhia;Codigo_CVM;Data_Referencia;Categoria;Tipo;Especie;Assunto;Data_Entrega;Tipo_Apresentacao;Protocolo_Entrega;Versao;Link_Download\r\n" +
	"00.000.000/0001-91;BANCO DO BRASIL S.A.;1023;2026-01-19;Fato Relevante;;;Payout 2026;2026-01-19;AP - Apresentacao;001023IPE190120260001;1;https://rad.cvm.gov.br/link1\r\n" +
	"00.000.000/0001-91;BANCO DO BRASIL S.A.;1023;2026-01-20;Comunicado ao Mercado;;;Aviso;2026-01-20;AP - Apresentacao;001023IPE200120260002;1;https://rad.cvm.gov.br/link2\r\n" +
	"11.111.111/0001-11;PETROBRAS S.A.;9512;2026-02-10;Fato Relevante;;;Descoberta de reservas;2026-02-10;AP - Apresentacao;009512IPE100220260003;1;https://rad.cvm.gov.br/link3\r\n"

func newFatosServer(t *testing.T, csvContent string) *httptest.Server {
	t.Helper()
	zipData := buildFatosZip(t, csvContent)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipData)
	}))
}

func TestFatosRelevantesCollector_Source(t *testing.T) {
	srv := newFatosServer(t, fakeFatosCSV)
	defer srv.Close()

	c := cvm.NewFatosRelevantesCollector(srv.URL)
	if got := c.Source(); got != "cvm_fatos" {
		t.Errorf("Source() = %q, want cvm_fatos", got)
	}
}

func TestFatosRelevantesCollector_Schedule(t *testing.T) {
	srv := newFatosServer(t, fakeFatosCSV)
	defer srv.Close()

	c := cvm.NewFatosRelevantesCollector(srv.URL)
	if got := c.Schedule(); got != "@monthly" {
		t.Errorf("Schedule() = %q, want @monthly", got)
	}
}

func TestFatosRelevantesCollector_Collect_OnlyFatos(t *testing.T) {
	srv := newFatosServer(t, fakeFatosCSV)
	defer srv.Close()

	c := cvm.NewFatosRelevantesCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Should return exactly 2 fatos relevantes (not the Comunicado ao Mercado row).
	if len(records) != 2 {
		t.Fatalf("expected 2 records (only fatos relevantes), got %d", len(records))
	}

	for _, r := range records {
		if r.Source != "cvm_fatos" {
			t.Errorf("Source = %q, want cvm_fatos", r.Source)
		}
		if r.RecordKey == "" {
			t.Error("RecordKey must not be empty")
		}
		cat, _ := r.Data["categoria"].(string)
		if cat != "Fato Relevante" {
			t.Errorf("categoria = %q, want Fato Relevante", cat)
		}
		for _, field := range []string{"cnpj", "empresa", "protocolo", "data_entrega", "link_download", "assunto"} {
			if _, ok := r.Data[field]; !ok {
				t.Errorf("record %q missing field %q", r.RecordKey, field)
			}
		}
	}
}

func TestFatosRelevantesCollector_Collect_RecordKey_IsProtocolo(t *testing.T) {
	srv := newFatosServer(t, fakeFatosCSV)
	defer srv.Close()

	c := cvm.NewFatosRelevantesCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	wantKeys := map[string]bool{
		"001023IPE190120260001": true,
		"009512IPE100220260003": true,
	}
	for _, r := range records {
		if !wantKeys[r.RecordKey] {
			t.Errorf("unexpected RecordKey %q", r.RecordKey)
		}
		proto, _ := r.Data["protocolo"].(string)
		if proto != r.RecordKey {
			t.Errorf("protocolo field %q != RecordKey %q", proto, r.RecordKey)
		}
	}
}

func TestFatosRelevantesCollector_Collect_EmptyCSV(t *testing.T) {
	headerOnly := "CNPJ_Companhia;Nome_Companhia;Codigo_CVM;Data_Referencia;Categoria;Tipo;Especie;Assunto;Data_Entrega;Tipo_Apresentacao;Protocolo_Entrega;Versao;Link_Download\r\n"
	srv := newFatosServer(t, headerOnly)
	defer srv.Close()

	c := cvm.NewFatosRelevantesCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for header-only CSV, got %d", len(records))
	}
}

func TestFatosRelevantesCollector_Collect_NoFatosRelevantes(t *testing.T) {
	noFatos := "CNPJ_Companhia;Nome_Companhia;Codigo_CVM;Data_Referencia;Categoria;Tipo;Especie;Assunto;Data_Entrega;Tipo_Apresentacao;Protocolo_Entrega;Versao;Link_Download\r\n" +
		"00.000.000/0001-91;BANCO DO BRASIL S.A.;1023;2026-01-20;Comunicado ao Mercado;;;Aviso;2026-01-20;AP - Apresentacao;001023IPE200120260002;1;https://rad.cvm.gov.br/link2\r\n" +
		"00.000.000/0001-91;BANCO DO BRASIL S.A.;1023;2026-01-21;Assembleia;;;Convocacao;2026-01-21;AP - Apresentacao;001023IPE210120260003;1;https://rad.cvm.gov.br/link3\r\n"
	srv := newFatosServer(t, noFatos)
	defer srv.Close()

	c := cvm.NewFatosRelevantesCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 fato relevante records, got %d", len(records))
	}
}

func TestFatosRelevantesCollector_Collect_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := cvm.NewFatosRelevantesCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected an error for HTTP 500, got nil")
	}
}
