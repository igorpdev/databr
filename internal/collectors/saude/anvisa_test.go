package saude_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/saude"
)

// minimalCSV contains a header row plus two active medicamento rows and one inactive row.
// Uses semicolon delimiters and ISO-8859-1 encoding conventions (ASCII-safe for test data).
// Columns: TIPO_PRODUTO;NOME_PRODUTO;DATA_FINALIZACAO_PROCESSO;CATEGORIA_REGULATORIA;
//
//	NUMERO_REGISTRO_PRODUTO;DATA_VENCIMENTO_REGISTRO;NUMERO_PROCESSO;
//	CLASSE_TERAPEUTICA;EMPRESA_DETENTORA_REGISTRO;SITUACAO_REGISTRO;PRINCIPIO_ATIVO
var minimalCSV = `TIPO_PRODUTO;NOME_PRODUTO;DATA_FINALIZACAO_PROCESSO;CATEGORIA_REGULATORIA;NUMERO_REGISTRO_PRODUTO;DATA_VENCIMENTO_REGISTRO;NUMERO_PROCESSO;CLASSE_TERAPEUTICA;EMPRESA_DETENTORA_REGISTRO;SITUACAO_REGISTRO;PRINCIPIO_ATIVO
"MEDICAMENTO";"ACERATUM";"21/02/2003";"SIMILAR";104400005;"01/06/2025";"2599200340370";"MEDICAMENTOS ATIVOS NA SECRECAO GORDUROSA";"33173097000274 - CELLERA FARMACEUTICA S.A.";"VALIDO";"PEROXIDO DE UREIA"
"MEDICAMENTO";"ASPIRINA";"10/01/2005";"REFERENCIA";123456789;"01/12/2028";"1234567890123";"ANALGÉSICOS";"12345678000199 - BAYER S.A.";"ATIVO";"ACIDO ACETILSALICILICO"
"MEDICAMENTO";"PRODUTO VENCIDO";"01/01/2000";"SIMILAR";999999999;"01/01/2010";"0000000000000";"VITAMINAS";"00000000000000 - EMPRESA VELHA LTDA";"CADUCO/CANCELADO";"VITAMINA C"
`

var headerOnlyCSV = `TIPO_PRODUTO;NOME_PRODUTO;DATA_FINALIZACAO_PROCESSO;CATEGORIA_REGULATORIA;NUMERO_REGISTRO_PRODUTO;DATA_VENCIMENTO_REGISTRO;NUMERO_PROCESSO;CLASSE_TERAPEUTICA;EMPRESA_DETENTORA_REGISTRO;SITUACAO_REGISTRO;PRINCIPIO_ATIVO
`

func newAnvisaServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
}

func TestAnvisaCollector_Source(t *testing.T) {
	srv := newAnvisaServer(t, minimalCSV)
	defer srv.Close()

	c := saude.NewAnvisaCollector(srv.URL)
	if got := c.Source(); got != "anvisa_medicamentos" {
		t.Errorf("Source() = %q, want %q", got, "anvisa_medicamentos")
	}
}

func TestAnvisaCollector_Schedule(t *testing.T) {
	srv := newAnvisaServer(t, minimalCSV)
	defer srv.Close()

	c := saude.NewAnvisaCollector(srv.URL)
	if got := c.Schedule(); got != "0 12 15 * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 12 15 * *")
	}
}

func TestAnvisaCollector_Collect(t *testing.T) {
	srv := newAnvisaServer(t, minimalCSV)
	defer srv.Close()

	c := saude.NewAnvisaCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Only the two active records should be returned (VALIDO + ATIVO), not CADUCO/CANCELADO.
	if len(records) != 2 {
		t.Fatalf("Collect() returned %d records, want 2", len(records))
	}

	r := records[0]
	if r.Source != "anvisa_medicamentos" {
		t.Errorf("Source = %q, want %q", r.Source, "anvisa_medicamentos")
	}
	if r.RecordKey == "" {
		t.Error("RecordKey must not be empty (should be NUMERO_REGISTRO_PRODUTO)")
	}

	// Verify required data fields are present.
	requiredFields := []string{"produto", "empresa", "situacao", "data_vencimento", "classe_terapeutica", "principio_ativo", "categoria"}
	for _, field := range requiredFields {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data must contain %q field; got keys: %v", field, dataKeys(r.Data))
		}
	}

	// RecordKey for first record should be the registration number as string.
	if r.RecordKey != "104400005" {
		t.Errorf("RecordKey = %q, want %q", r.RecordKey, "104400005")
	}
}

func TestAnvisaCollector_EmptyCSV(t *testing.T) {
	srv := newAnvisaServer(t, headerOnlyCSV)
	defer srv.Close()

	c := saude.NewAnvisaCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() on empty CSV should not error, got: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Collect() on empty CSV returned %d records, want 0", len(records))
	}
}

func TestAnvisaCollector_FilterInactive(t *testing.T) {
	srv := newAnvisaServer(t, minimalCSV)
	defer srv.Close()

	c := saude.NewAnvisaCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Ensure no CADUCO/CANCELADO record is included.
	for _, rec := range records {
		situacao, _ := rec.Data["situacao"].(string)
		if situacao == "CADUCO/CANCELADO" {
			t.Errorf("inactive record included: %q", rec.RecordKey)
		}
	}
}

func dataKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
