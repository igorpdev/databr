package tse_test

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/tse"
)

const fakeCandidatosCSV = `SQ_CANDIDATO;NM_CANDIDATO;SG_PARTIDO;DS_CARGO;SG_UF;NR_CPF_CANDIDATO
123456;JOAO DA SILVA;PT;DEPUTADO ESTADUAL;SP;12345678900
789012;MARIA SOUZA;PSDB;VEREADOR;RJ;98765432100`

func buildTSEZip(t *testing.T, csvContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("consulta_cand_2024_BR.csv")
	f.Write([]byte(csvContent))
	w.Close()
	return buf.Bytes()
}

func newTSEServer(t *testing.T) *httptest.Server {
	t.Helper()
	zipData := buildTSEZip(t, fakeCandidatosCSV)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipData)
	}))
}

func TestCandidatosCollector_Source(t *testing.T) {
	srv := newTSEServer(t)
	defer srv.Close()
	c := tse.NewCandidatosCollector(srv.URL)
	if c.Source() != "tse_candidatos" {
		t.Errorf("Source() = %q, want tse_candidatos", c.Source())
	}
}

func TestCandidatosCollector_Schedule(t *testing.T) {
	srv := newTSEServer(t)
	defer srv.Close()
	c := tse.NewCandidatosCollector(srv.URL)
	// TSE data is per election, not scheduled regularly
	if c.Schedule() != "@yearly" {
		t.Errorf("Schedule() = %q, want @yearly", c.Schedule())
	}
}

func TestCandidatosCollector_Collect(t *testing.T) {
	srv := newTSEServer(t)
	defer srv.Close()

	c := tse.NewCandidatosCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Source != "tse_candidatos" {
		t.Errorf("Source = %q, want tse_candidatos", r.Source)
	}
	for _, field := range []string{"nm_candidato", "sg_partido", "ds_cargo", "sg_uf"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}
