package energia_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/energia"
)

// fakeCSV mimics the semicolon-separated, quoted, ISO-8859-1-like format returned by
// the ANEEL CKAN download endpoint (using ASCII-safe content for the stub).
const fakeCSV = `"DatGeracaoConjuntoDados";"DscREH";"SigAgente";"NumCNPJDistribuidora";"DatInicioVigencia";"DatFimVigencia";"DscBaseTarifaria";"DscSubGrupo";"DscModalidadeTarifaria";"DscClasse";"DscSubClasse";"DscDetalhe";"NomPostoTarifario";"DscUnidadeTerciaria";"SigAgenteAcessante";"VlrTUSD";"VlrTE"
"2026-02-22";"RESOLUCAO HOMOLOGATORIA N 0.937";"CPFL JAGUARI";"53859112000169";"2025-02-01";"2026-01-31";"Tarifa de Aplicacao";"B1";"Convencional";"Residencial";"Nao se aplica";"Nao se aplica";"Nao se aplica";"MWh";"Nao se aplica";"0,00";"999,00"
"2026-02-22";"RESOLUCAO HOMOLOGATORIA N 1.234";"ENEL SP";"61695227000193";"2025-01-01";"2025-12-31";"Tarifa de Aplicacao";"B1";"Convencional";"Residencial";"Nao se aplica";"Nao se aplica";"Nao se aplica";"MWh";"Nao se aplica";"0,00";"850,50"
`

func newCSVServer(t *testing.T, body string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
}

func TestANEELCollector_Source(t *testing.T) {
	srv := newCSVServer(t, fakeCSV, http.StatusOK)
	defer srv.Close()

	c := energia.NewANEELCollector(srv.URL)
	if got := c.Source(); got != "aneel_tarifas" {
		t.Errorf("Source() = %q, want %q", got, "aneel_tarifas")
	}
}

func TestANEELCollector_Schedule(t *testing.T) {
	srv := newCSVServer(t, fakeCSV, http.StatusOK)
	defer srv.Close()

	c := energia.NewANEELCollector(srv.URL)
	if got := c.Schedule(); got != "0 6 * * 1" {
		t.Errorf("Schedule() = %q, want %q", got, "0 6 * * 1")
	}
}

func TestANEELCollector_Collect(t *testing.T) {
	srv := newCSVServer(t, fakeCSV, http.StatusOK)
	defer srv.Close()

	c := energia.NewANEELCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Collect() returned 0 records, want > 0")
	}

	r := records[0]
	if r.Source != "aneel_tarifas" {
		t.Errorf("Source = %q, want aneel_tarifas", r.Source)
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty")
	}
	if _, ok := r.Data["distribuidora"]; !ok {
		t.Error("Data must contain 'distribuidora' field")
	}
	if _, ok := r.Data["cnpj"]; !ok {
		t.Error("Data must contain 'cnpj' field")
	}
	if _, ok := r.Data["vlr_tusd"]; !ok {
		t.Error("Data must contain 'vlr_tusd' field")
	}
	if _, ok := r.Data["vlr_te"]; !ok {
		t.Error("Data must contain 'vlr_te' field")
	}
	if _, ok := r.Data["dat_inicio_vigencia"]; !ok {
		t.Error("Data must contain 'dat_inicio_vigencia' field")
	}
}

func TestANEELCollector_EmptyResult(t *testing.T) {
	// Only header row, no data rows.
	headerOnly := `"DatGeracaoConjuntoDados";"DscREH";"SigAgente";"NumCNPJDistribuidora";"DatInicioVigencia";"DatFimVigencia";"DscBaseTarifaria";"DscSubGrupo";"DscModalidadeTarifaria";"DscClasse";"DscSubClasse";"DscDetalhe";"NomPostoTarifario";"DscUnidadeTerciaria";"SigAgenteAcessante";"VlrTUSD";"VlrTE"` + "\n"

	srv := newCSVServer(t, headerOnly, http.StatusOK)
	defer srv.Close()

	c := energia.NewANEELCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() with no rows should not error, got: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for header-only CSV, got %d", len(records))
	}
}

func TestANEELCollector_HTTPError(t *testing.T) {
	srv := newCSVServer(t, "server error", http.StatusInternalServerError)
	defer srv.Close()

	c := energia.NewANEELCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500 response, got nil")
	}
}

func TestANEELCollector_RecordKeyIsUnique(t *testing.T) {
	srv := newCSVServer(t, fakeCSV, http.StatusOK)
	defer srv.Close()

	c := energia.NewANEELCollector(srv.URL)
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
