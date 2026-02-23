package transporte_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/collectors/transporte"
)

// fakeRNTRCCSV is a small but representative RNTRC CSV snippet with:
//   - A header row (unquoted)
//   - Two data rows: one ETC company (CNPJ), one TAC individual (CPF-like)
//   - A row with an empty rntrc to exercise the skip-logic
//   - A short/malformed row to exercise the column-count guard
const fakeRNTRCCSV = `nome_transportador;numero_rntrc;data_primeiro_cadastro;situacao_rntrc;cpfcnpjtransportador;categoria_transportador;cep;municipio;uf;equiparado;data_situacao_rntrc
"EMPRESA DE TRANSPORTE TESTE LTDA";"050085788";"23/05/2017";"ATIVO";"11.193.322/0001-10";"ETC";"14095-290";"RIBEIRAO PRETO";"SP";"Sim";"23/10/2024"
"JOAO DA SILVA TRANSPORTE";"012345678";"15/03/2010";"ATIVO";"123.456.789-09";"TAC";"01310-100";"SAO PAULO";"SP";"Nao";"01/01/2025"
"EMPRESA SEM RNTRC";"";;"ATIVO";"22.333.444/0001-55";"ETC";"00000-000";"BRASILIA";"DF";"Nao";"01/01/2025"
"LINHA CURTA";"999999999"
`

// buildFakeCKANResponse creates a minimal CKAN package_show JSON response that points
// to the provided csvServerURL as its only CSV resource.
func buildFakeCKANResponse(csvServerURL string) []byte {
	type resource struct {
		URL    string `json:"url"`
		Name   string `json:"name"`
		Format string `json:"format"`
	}
	type result struct {
		Resources []resource `json:"resources"`
	}
	type pkg struct {
		Result result `json:"result"`
	}

	p := pkg{
		Result: result{
			Resources: []resource{
				{URL: "https://example.com/dicionario.pdf", Name: "Dicionario de Dados", Format: "PDF"},
				{URL: csvServerURL + "/transportadores_rntrc_01_2026.csv", Name: "Jan26 - RNTRC", Format: "CSV"},
			},
		},
	}
	b, _ := json.Marshal(p)
	return b
}

// newMockCKANServer starts an httptest.Server that responds to all requests with the
// given CKAN JSON body and the provided status code.
func newMockCKANServer(t *testing.T, body []byte, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
	}))
}

// newMockCSVServer starts an httptest.Server that serves the given CSV body.
func newMockCSVServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
}

func TestANTTCollector_Source(t *testing.T) {
	c := transporte.NewANTTCollector("", "")
	if got := c.Source(); got != "antt_rntrc" {
		t.Errorf("Source() = %q, want %q", got, "antt_rntrc")
	}
}

func TestANTTCollector_Schedule(t *testing.T) {
	c := transporte.NewANTTCollector("", "")
	if got := c.Schedule(); got != "@monthly" {
		t.Errorf("Schedule() = %q, want %q", got, "@monthly")
	}
}

func TestANTTCollector_Collect_DirectCSVURL(t *testing.T) {
	csvSrv := newMockCSVServer(t, fakeRNTRCCSV, http.StatusOK)
	defer csvSrv.Close()

	// Pass csvURL directly — bypasses CKAN discovery.
	c := transporte.NewANTTCollector("", csvSrv.URL+"/transportadores_rntrc_01_2026.csv")
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Expect 2 valid records (empty rntrc and short row are skipped).
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// --- first record (ETC company) ---
	r0 := records[0]
	if r0.Source != "antt_rntrc" {
		t.Errorf("records[0].Source = %q, want %q", r0.Source, "antt_rntrc")
	}
	if r0.RecordKey != "050085788" {
		t.Errorf("records[0].RecordKey = %q, want %q", r0.RecordKey, "050085788")
	}
	if r0.Data["situacao"] != "ATIVO" {
		t.Errorf("records[0].Data[situacao] = %q, want ATIVO", r0.Data["situacao"])
	}
	if r0.Data["categoria"] != "ETC" {
		t.Errorf("records[0].Data[categoria] = %q, want ETC", r0.Data["categoria"])
	}

	// CNPJ normalization: "11.193.322/0001-10" → "11193322000110"
	if got, want := r0.Data["cpf_cnpj_digits"], "11193322000110"; got != want {
		t.Errorf("records[0].Data[cpf_cnpj_digits] = %q, want %q", got, want)
	}
	if got := r0.Data["cpf_cnpj"]; got != "11.193.322/0001-10" {
		t.Errorf("records[0].Data[cpf_cnpj] = %q, want formatted CNPJ", got)
	}

	// Location fields
	if r0.Data["uf"] != "SP" {
		t.Errorf("records[0].Data[uf] = %q, want SP", r0.Data["uf"])
	}
	if r0.Data["municipio"] != "RIBEIRAO PRETO" {
		t.Errorf("records[0].Data[municipio] = %q, want RIBEIRAO PRETO", r0.Data["municipio"])
	}

	// --- second record (TAC individual) ---
	r1 := records[1]
	if r1.RecordKey != "012345678" {
		t.Errorf("records[1].RecordKey = %q, want %q", r1.RecordKey, "012345678")
	}
	if r1.Data["categoria"] != "TAC" {
		t.Errorf("records[1].Data[categoria] = %q, want TAC", r1.Data["categoria"])
	}
	// CPF normalization: "123.456.789-09" → "12345678909"
	if got, want := r1.Data["cpf_cnpj_digits"], "12345678909"; got != want {
		t.Errorf("records[1].Data[cpf_cnpj_digits] = %q, want %q", got, want)
	}
}

func TestANTTCollector_Collect_CKANDiscovery(t *testing.T) {
	csvSrv := newMockCSVServer(t, fakeRNTRCCSV, http.StatusOK)
	defer csvSrv.Close()

	ckanBody := buildFakeCKANResponse(csvSrv.URL)
	ckanSrv := newMockCKANServer(t, ckanBody, http.StatusOK)
	defer ckanSrv.Close()

	// No csvURL — collector must discover it from CKAN.
	c := transporte.NewANTTCollector(ckanSrv.URL, "")
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() via CKAN discovery error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records via CKAN discovery, got %d", len(records))
	}
}

func TestANTTCollector_Collect_CKANFailure_ReturnsEmpty(t *testing.T) {
	// CKAN returns 500 — collector should log a warning and return empty (not error).
	ckanSrv := newMockCKANServer(t, []byte(`{"error": "server error"}`), http.StatusInternalServerError)
	defer ckanSrv.Close()

	c := transporte.NewANTTCollector(ckanSrv.URL, "")
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("expected nil error when CKAN fails, got: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil records when CKAN fails, got %d", len(records))
	}
}

func TestANTTCollector_Collect_CSVHTTPError(t *testing.T) {
	csvSrv := newMockCSVServer(t, "error", http.StatusInternalServerError)
	defer csvSrv.Close()

	c := transporte.NewANTTCollector("", csvSrv.URL+"/file.csv")
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 500 CSV response, got nil")
	}
}

func TestANTTCollector_Collect_EmptyCSV(t *testing.T) {
	headerOnly := "nome_transportador;numero_rntrc;data_primeiro_cadastro;situacao_rntrc;cpfcnpjtransportador;categoria_transportador;cep;municipio;uf;equiparado;data_situacao_rntrc\n"
	csvSrv := newMockCSVServer(t, headerOnly, http.StatusOK)
	defer csvSrv.Close()

	c := transporte.NewANTTCollector("", csvSrv.URL+"/file.csv")
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("header-only CSV should not return error, got: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for header-only CSV, got %d", len(records))
	}
}

func TestANTTCollector_RecordKey_IsRNTRC(t *testing.T) {
	csvSrv := newMockCSVServer(t, fakeRNTRCCSV, http.StatusOK)
	defer csvSrv.Close()

	c := transporte.NewANTTCollector("", csvSrv.URL+"/file.csv")
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, r := range records {
		if r.RecordKey == "" {
			t.Error("RecordKey must not be empty")
		}
		// RecordKey must match the rntrc field in Data.
		if r.RecordKey != r.Data["rntrc"] {
			t.Errorf("RecordKey %q != Data[rntrc] %q", r.RecordKey, r.Data["rntrc"])
		}
	}
}

func TestANTTCollector_RequiredDataFields(t *testing.T) {
	csvSrv := newMockCSVServer(t, fakeRNTRCCSV, http.StatusOK)
	defer csvSrv.Close()

	c := transporte.NewANTTCollector("", csvSrv.URL+"/file.csv")
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("no records returned")
	}

	required := []string{
		"nome", "rntrc", "situacao", "categoria",
		"cpf_cnpj", "cpf_cnpj_digits",
		"municipio", "uf", "cep",
		"data_cadastro", "data_situacao", "equiparado",
	}
	for _, key := range required {
		if _, ok := records[0].Data[key]; !ok {
			t.Errorf("Data must contain %q field", key)
		}
	}
}

func TestANTTCollector_CKANNoCsvResource_ReturnsEmpty(t *testing.T) {
	// CKAN returns a valid response but with no CSV resources.
	body := []byte(`{"result":{"resources":[{"url":"https://example.com/dash","name":"Dashboard","format":"HTML"}]}}`)
	ckanSrv := newMockCKANServer(t, body, http.StatusOK)
	defer ckanSrv.Close()

	c := transporte.NewANTTCollector(ckanSrv.URL, "")
	records, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("expected nil error when no CSV in CKAN, got: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil records when no CSV in CKAN, got %d", len(records))
	}
}
