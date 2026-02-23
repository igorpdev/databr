package ans_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/ans"
	"github.com/databr/api/internal/testutil"
)

var testCSV = "Registro_ANS;CNPJ;Razao_Social;Nome_Fantasia;Modalidade;Logradouro;Numero;Complemento;Bairro;Cidade;UF;CEP;DDD;Telefone;Fax;Endereco_eletronico;Representante;Cargo_Representante;Regiao_de_Comercializacao;Data_Registro_ANS\n" +
	"312126;92693118000160;UNIMED PORTO ALEGRE;UNIMED PORTO ALEGRE;Cooperativa Medica;Rua ABC;123;;Centro;Porto Alegre;RS;90000000;51;33333333;;email@unimed.com.br;FULANO;Presidente;RS;01/01/2000\n" +
	"326305;01234567000199;AMIL ASSISTENCIA;AMIL;Medicina de Grupo;Av XYZ;456;;Centro;Sao Paulo;SP;01000000;11;44444444;;email@amil.com.br;CICLANO;Diretor;SP;02/02/2005\n"

var headerOnlyCSV = "Registro_ANS;CNPJ;Razao_Social;Nome_Fantasia;Modalidade;Logradouro;Numero;Complemento;Bairro;Cidade;UF;CEP;DDD;Telefone;Fax;Endereco_eletronico;Representante;Cargo_Representante;Regiao_de_Comercializacao;Data_Registro_ANS\n"

func newANSServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
}

func TestOperadorasCollector_Source(t *testing.T) {
	srv := newANSServer(t, testCSV)
	defer srv.Close()

	c := ans.NewOperadorasCollector(srv.URL)
	if got := c.Source(); got != "ans_operadoras" {
		t.Errorf("Source() = %q, want %q", got, "ans_operadoras")
	}
}

func TestOperadorasCollector_Schedule(t *testing.T) {
	srv := newANSServer(t, testCSV)
	defer srv.Close()

	c := ans.NewOperadorasCollector(srv.URL)
	if got := c.Schedule(); got != "@daily" {
		t.Errorf("Schedule() = %q, want %q", got, "@daily")
	}
}

func TestOperadorasCollector_Collect(t *testing.T) {
	srv := newANSServer(t, testCSV)
	defer srv.Close()

	c := ans.NewOperadorasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Collect() returned %d records, want 2", len(records))
	}

	r := records[0]
	if r.Source != "ans_operadoras" {
		t.Errorf("Source = %q, want %q", r.Source, "ans_operadoras")
	}
	if r.RecordKey != "312126" {
		t.Errorf("RecordKey = %q, want %q", r.RecordKey, "312126")
	}

	// Verify required data fields are present with normalized keys.
	requiredFields := []string{
		"registro_ans", "cnpj", "razao_social", "nome_fantasia",
		"modalidade", "cidade", "uf", "data_registro_ans",
	}
	for _, field := range requiredFields {
		if _, ok := r.Data[field]; !ok {
			t.Errorf("Data must contain %q field; got keys: %v", field, testutil.DataKeys(r.Data))
		}
	}

	// Verify specific values.
	if got, _ := r.Data["cnpj"].(string); got != "92693118000160" {
		t.Errorf("Data[cnpj] = %q, want %q", got, "92693118000160")
	}
	if got, _ := r.Data["razao_social"].(string); got != "UNIMED PORTO ALEGRE" {
		t.Errorf("Data[razao_social] = %q, want %q", got, "UNIMED PORTO ALEGRE")
	}

	// Verify second record.
	r2 := records[1]
	if r2.RecordKey != "326305" {
		t.Errorf("RecordKey[1] = %q, want %q", r2.RecordKey, "326305")
	}
}

func TestOperadorasCollector_EmptyCSV(t *testing.T) {
	srv := newANSServer(t, headerOnlyCSV)
	defer srv.Close()

	c := ans.NewOperadorasCollector(srv.URL)
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() on empty CSV should not error, got: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Collect() on empty CSV returned %d records, want 0", len(records))
	}
}

func TestOperadorasCollector_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := ans.NewOperadorasCollector(srv.URL)
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("Collect() should return error on HTTP 500")
	}
}

func TestOperadorasCollector_DefaultURL(t *testing.T) {
	// Passing empty string should use the default URL (not panic).
	c := ans.NewOperadorasCollector("")
	if c.Source() != "ans_operadoras" {
		t.Errorf("Source() = %q, want %q", c.Source(), "ans_operadoras")
	}
}

