package transporte_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/transporte"
)

// fakePRFCSV is a minimal semicolon-delimited CSV in the PRF datatran format.
// Encoding is UTF-8 in tests (the collector transcodes ISO-8859-1 → UTF-8, which is
// a no-op for ASCII data).
const fakePRFCSV = "id;data_inversa;dia_semana;horario;uf;br;km;municipio;causa_acidente;tipo_acidente;classificacao_acidente;fase_dia;sentido_via;condicao_metereologica;tipo_pista;tracado_via;uso_solo;pessoas;mortos;feridos_leves;feridos_graves;ilesos;ignorados;feridos;veiculos\r\n" +
	"100001;2026-01-15;Quarta-feira;14:30:00;MG;040;123.4;BELO HORIZONTE;Falta de atencao;Colisao traseira;Com Vitimas Feridas;Pleno dia;Crescente;Ceu Claro;Simples;Reta;Sim;3;0;1;0;2;0;1;2\r\n" +
	"100002;2026-01-15;Quarta-feira;16:45:00;SP;116;45.2;GUARULHOS;Velocidade incompativel;Saida de leito carrocarvel;Com Vitimas Fatais;Pleno dia;Decrescente;Chuva;Dupla;Curva;Nao;2;1;0;1;0;0;1;1\r\n" +
	";2026-01-15;Quarta-feira;18:00:00;RJ;101;10.0;RIO DE JANEIRO;Outras;Colisao lateral;Sem Vitimas;Anoitecer;Crescente;Ceu Claro;Simples;Reta;Sim;2;0;0;0;2;0;0;2\r\n"

func newPRFServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=iso-8859-1")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
}

func TestPRFCollector_Source(t *testing.T) {
	c := transporte.NewPRFCollector("http://localhost")
	if got := c.Source(); got != "prf_acidentes" {
		t.Errorf("Source() = %q, want %q", got, "prf_acidentes")
	}
}

func TestPRFCollector_Schedule(t *testing.T) {
	c := transporte.NewPRFCollector("http://localhost")
	if got := c.Schedule(); got != "0 8 1 * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 8 1 * *")
	}
}

func TestPRFCollector_Collect(t *testing.T) {
	srv := newPRFServer(t, fakePRFCSV, http.StatusOK)
	defer srv.Close()

	c := transporte.NewPRFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Expect 2 valid records (row with empty id is skipped).
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestPRFCollector_Collect_RecordKey(t *testing.T) {
	srv := newPRFServer(t, fakePRFCSV, http.StatusOK)
	defer srv.Close()

	c := transporte.NewPRFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if records[0].RecordKey != "100001" {
		t.Errorf("records[0].RecordKey = %q, want %q", records[0].RecordKey, "100001")
	}
	if records[1].RecordKey != "100002" {
		t.Errorf("records[1].RecordKey = %q, want %q", records[1].RecordKey, "100002")
	}
}

func TestPRFCollector_Collect_SourceField(t *testing.T) {
	srv := newPRFServer(t, fakePRFCSV, http.StatusOK)
	defer srv.Close()

	c := transporte.NewPRFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.Source != "prf_acidentes" {
			t.Errorf("Source = %q, want %q", rec.Source, "prf_acidentes")
		}
	}
}

func TestPRFCollector_Collect_DataFields(t *testing.T) {
	srv := newPRFServer(t, fakePRFCSV, http.StatusOK)
	defer srv.Close()

	c := transporte.NewPRFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one record")
	}

	rec := records[0]
	required := []string{
		"id", "data_inversa", "dia_semana", "horario", "uf", "br", "km",
		"municipio", "causa_acidente", "tipo_acidente", "classificacao_acidente",
		"fase_dia", "sentido_via", "condicao_metereologica", "tipo_pista",
		"tracado_via", "uso_solo", "pessoas", "mortos", "feridos_leves",
		"feridos_graves", "ilesos", "ignorados", "feridos", "veiculos",
	}
	for _, field := range required {
		if _, ok := rec.Data[field]; !ok {
			t.Errorf("Data must contain field %q", field)
		}
	}

	// Spot-check values.
	if got, _ := rec.Data["uf"].(string); got != "MG" {
		t.Errorf("uf = %q, want MG", got)
	}
	if got, _ := rec.Data["municipio"].(string); got != "BELO HORIZONTE" {
		t.Errorf("municipio = %q, want BELO HORIZONTE", got)
	}
	if got, _ := rec.Data["mortos"].(string); got != "0" {
		t.Errorf("mortos = %q, want 0", got)
	}
}

func TestPRFCollector_Collect_SkipsEmptyID(t *testing.T) {
	srv := newPRFServer(t, fakePRFCSV, http.StatusOK)
	defer srv.Close()

	c := transporte.NewPRFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, rec := range records {
		if rec.RecordKey == "" {
			t.Error("found record with empty RecordKey; should have been skipped")
		}
	}
}

func TestPRFCollector_Collect_HTTPError(t *testing.T) {
	srv := newPRFServer(t, "error", http.StatusInternalServerError)
	defer srv.Close()

	c := transporte.NewPRFCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestPRFCollector_Collect_EmptyCSV(t *testing.T) {
	headerOnly := "id;data_inversa;dia_semana;horario;uf;br;km;municipio;causa_acidente;tipo_acidente;classificacao_acidente;fase_dia;sentido_via;condicao_metereologica;tipo_pista;tracado_via;uso_solo;pessoas;mortos;feridos_leves;feridos_graves;ilesos;ignorados;feridos;veiculos\r\n"
	srv := newPRFServer(t, headerOnly, http.StatusOK)
	defer srv.Close()

	c := transporte.NewPRFCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("header-only CSV should not return error, got: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}
