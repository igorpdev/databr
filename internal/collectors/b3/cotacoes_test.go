package b3_test

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/databr/api/internal/collectors/b3"
)

// BDIN format: fixed-width text. Record type 01 = header, 02 = data, 99 = trailer.
// Record type 02 has ticker at positions 12-24 and close price at 108-121.
// Full spec: https://bvmf.bmfbovespa.com.br/InstDados/SerHist/BDIN_Layout.pdf
const fakeBDINData = `0120260220BOVESPA 20260220              12345678
0220260220BDICOTAH PETR4       010PETROBRAS PN         R$  000000003250900000000029500000000030000000000032999000000000295000000000032000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000100000000000000000000000000000000000PETROBRAS PN                                    10300000000000000000000
9920260220000001000000000001
`

func buildB3Zip(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("COTAHIST_D20022026.TXT")
	f.Write([]byte(content))
	w.Close()
	return buf.Bytes()
}

func newB3Server(t *testing.T) *httptest.Server {
	t.Helper()
	zipData := buildB3Zip(t, fakeBDINData)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipData)
	}))
}

func TestCotacoesCollector_Source(t *testing.T) {
	srv := newB3Server(t)
	defer srv.Close()
	c := b3.NewCotacoesCollector(srv.URL)
	if c.Source() != "b3_cotacoes" {
		t.Errorf("Source() = %q, want b3_cotacoes", c.Source())
	}
}

func TestCotacoesCollector_Schedule(t *testing.T) {
	srv := newB3Server(t)
	defer srv.Close()
	c := b3.NewCotacoesCollector(srv.URL)
	if c.Schedule() != "@daily" {
		t.Errorf("Schedule() = %q, want @daily", c.Schedule())
	}
}

func TestCotacoesCollector_Collect(t *testing.T) {
	srv := newB3Server(t)
	defer srv.Close()

	c := b3.NewCotacoesCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	r := records[0]
	if r.Source != "b3_cotacoes" {
		t.Errorf("Source = %q, want b3_cotacoes", r.Source)
	}
	for _, field := range []string{"ticker", "data_pregao"} {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data missing field %q", field)
		}
	}
}

func TestLastBusinessDay(t *testing.T) {
	// Monday 2026-02-23 → same day
	monday := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	got := b3.LastBusinessDay(monday)
	if got != monday {
		t.Errorf("Monday should be its own last business day, got %v", got)
	}

	// Sunday 2026-02-22 → Friday 2026-02-20
	sunday := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	got = b3.LastBusinessDay(sunday)
	expected := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	if got != expected {
		t.Errorf("Sunday last business day = %v, want %v", got, expected)
	}
}
