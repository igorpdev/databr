package handlers_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// buildTSEZip creates a minimal in-memory ZIP containing a single CSV file.
// The CSV uses ';' as separator and Latin-1-safe ASCII content (no special chars needed for tests).
func buildTSEZip(t *testing.T, filename, csvContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Create(filename)
	if err != nil {
		t.Fatalf("zip.Create: %v", err)
	}
	if _, err := fw.Write([]byte(csvContent)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// tseRedirectTransport rewrites every outgoing request to point at a test server,
// preserving the original path and query so that the handler URL-building logic works.
type tseRedirectTransport struct {
	target string // httptest server URL, e.g. "http://127.0.0.1:PORT"
}

func (t *tseRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	newReq.URL.Scheme = "http"
	newReq.URL.Host = t.target[len("http://"):]
	return http.DefaultTransport.RoundTrip(newReq)
}

// mockTSEExtras creates a TSEExtrasHandler whose HTTP client is redirected to srv.
func mockTSEExtras(t *testing.T, srv *httptest.Server) *handlers.TSEExtrasHandler {
	t.Helper()
	return handlers.NewTSEExtrasHandlerWithClient(
		&http.Client{Transport: &tseRedirectTransport{target: srv.URL}},
		srv.URL, // baseURL matches the test server so URL construction works
	)
}

// newTSEExtrasRouter registers all four TSE extras routes on a Chi router.
func newTSEExtrasRouter(h *handlers.TSEExtrasHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/eleicoes/bens", h.GetBens)
	r.Get("/v1/eleicoes/doacoes", h.GetDoacoes)
	r.Get("/v1/eleicoes/resultados", h.GetResultados)
	r.Get("/v1/energia/combustiveis", h.GetCombustiveis)
	return r
}

// -----------------------------------------------------------------------
// GetBens tests
// -----------------------------------------------------------------------

func TestTSEExtras_GetBens_OK(t *testing.T) {
	csvContent := "SQ_CANDIDATO;NM_CANDIDATO;VR_BEM_CANDIDATO\n" +
		"12345;JOAO DA SILVA;50000\n" +
		"67890;MARIA SOUZA;120000\n"
	zipBytes := buildTSEZip(t, "consulta_bem_candidato_2024_BRASIL.csv", csvContent)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write(zipBytes)
	}))
	defer srv.Close()

	h := mockTSEExtras(t, srv)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/bens?ano=2024&n=5", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "tse_bens" {
		t.Errorf("Source = %q, want tse_bens", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
	bens, ok := resp.Data["bens"].([]any)
	if !ok || len(bens) == 0 {
		t.Errorf("expected non-empty bens slice, got %T: %v", resp.Data["bens"], resp.Data["bens"])
	}
	// Verify that the first record has expected lower-cased column names.
	if len(bens) > 0 {
		row, ok := bens[0].(map[string]any)
		if !ok {
			t.Fatalf("bens[0] is not a map: %T", bens[0])
		}
		if _, hasKey := row["sq_candidato"]; !hasKey {
			t.Errorf("bens[0] missing key sq_candidato, got keys: %v", row)
		}
	}
}

func TestTSEExtras_GetBens_BadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := mockTSEExtras(t, srv)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/bens?ano=2024", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", rec.Code)
	}
}

func TestTSEExtras_GetBens_EmptyZip(t *testing.T) {
	// ZIP with no CSV files → 404
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.Close()
	emptyZip := buf.Bytes()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write(emptyZip)
	}))
	defer srv.Close()

	h := mockTSEExtras(t, srv)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/bens", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d: %s", rec.Code, rec.Body.String())
	}
}

// -----------------------------------------------------------------------
// GetDoacoes tests
// -----------------------------------------------------------------------

func TestTSEExtras_GetDoacoes_OK(t *testing.T) {
	csvContent := "SQ_CANDIDATO;NM_DOADOR;VR_RECEITA\n" +
		"11111;EMPRESA XYZ LTDA;10000\n"
	zipBytes := buildTSEZip(t, "receitas_candidatos_2024_BRASIL.csv", csvContent)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write(zipBytes)
	}))
	defer srv.Close()

	h := mockTSEExtras(t, srv)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/doacoes?ano=2024", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "tse_doacoes" {
		t.Errorf("Source = %q, want tse_doacoes", resp.Source)
	}
	doacoes, ok := resp.Data["doacoes"].([]any)
	if !ok || len(doacoes) == 0 {
		t.Errorf("expected non-empty doacoes slice, got %T", resp.Data["doacoes"])
	}
}

func TestTSEExtras_GetDoacoes_BadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	h := mockTSEExtras(t, srv)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/doacoes", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// GetResultados tests
// -----------------------------------------------------------------------

func TestTSEExtras_GetResultados_OK(t *testing.T) {
	csvContent := "SQ_CANDIDATO;NM_CANDIDATO;QT_VOTOS_NOMINAIS\n" +
		"99999;CANDIDATO A;5000\n" +
		"88888;CANDIDATO B;3000\n"
	zipBytes := buildTSEZip(t, "votacao_candidato_munzona_2024_BR.csv", csvContent)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write(zipBytes)
	}))
	defer srv.Close()

	h := mockTSEExtras(t, srv)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/resultados?ano=2024&n=10", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "tse_resultados" {
		t.Errorf("Source = %q, want tse_resultados", resp.Source)
	}
	resultados, ok := resp.Data["resultados"].([]any)
	if !ok || len(resultados) == 0 {
		t.Errorf("expected non-empty resultados slice, got %T", resp.Data["resultados"])
	}
}

func TestTSEExtras_GetResultados_BadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	h := mockTSEExtras(t, srv)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/resultados", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// GetCombustiveis tests
// -----------------------------------------------------------------------

// ipeaMultiSeriesHandler returns a minimal IPEADATA OData JSON response for any
// series code, returning two mock annual values.
func ipeaMultiSeriesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Minimal OData response: two values for whichever series was requested.
	resp := `{"value":[` +
		`{"SERCODIGO":"TEST","VALDATA":"2023-01-01T00:00:00-03:00","VALVALOR":3750.83,"NIVNOME":"","TERCODIGO":""},` +
		`{"SERCODIGO":"TEST","VALDATA":"2024-01-01T00:00:00-03:00","VALVALOR":3860.00,"NIVNOME":"","TERCODIGO":""}` +
		`]}`
	w.Write([]byte(resp))
}

func TestTSEExtras_GetCombustiveis_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(ipeaMultiSeriesHandler))
	defer srv.Close()

	h := handlers.NewTSEExtrasHandlerWithClient(
		&http.Client{Transport: &tseRedirectTransport{target: srv.URL}},
		srv.URL,
	)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/energia/combustiveis?n=2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "anp_combustiveis" {
		t.Errorf("Source = %q, want anp_combustiveis", resp.Source)
	}
	if resp.CostUSDC != "0.003" {
		t.Errorf("CostUSDC = %q, want 0.003", resp.CostUSDC)
	}
	combustiveis, ok := resp.Data["combustiveis"].([]any)
	if !ok || len(combustiveis) == 0 {
		t.Errorf("expected non-empty combustiveis slice, got %T", resp.Data["combustiveis"])
	}
	fonte, _ := resp.Data["fonte"].(string)
	if fonte != "ipeadata" {
		t.Errorf("fonte = %q, want ipeadata", fonte)
	}
}

func TestTSEExtras_GetCombustiveis_UpstreamError(t *testing.T) {
	// When IPEADATA returns 500, the handler should still return 200 but with error info
	// in the series entries (non-fatal, partial response). If ALL series fail, it returns 502.
	// Here we simulate 500 for all requests.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := handlers.NewTSEExtrasHandlerWithClient(
		&http.Client{Transport: &tseRedirectTransport{target: srv.URL}},
		srv.URL,
	)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/energia/combustiveis", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// The handler returns 200 with error info in each series entry
	// (all series fail gracefully, partial data response).
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (partial response) got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "anp_combustiveis" {
		t.Errorf("Source = %q, want anp_combustiveis", resp.Source)
	}
}

// -----------------------------------------------------------------------
// GetFiliados tests
// -----------------------------------------------------------------------

// mockTSEFiliadosHandler creates a TSEExtrasHandler whose filiados HTTP client
// is redirected to srv. filiadosURL points to the mock server's /filiados.zip path.
func mockTSEFiliadosHandler(t *testing.T, srv *httptest.Server) *handlers.TSEExtrasHandler {
	t.Helper()
	return handlers.NewTSEExtrasHandlerWithClientAndFiliados(
		&http.Client{Transport: &tseRedirectTransport{target: srv.URL}},
		srv.URL,
		srv.URL+"/filiados.zip",
	)
}

// newTSEFiliadosRouter registers filiados route on a Chi router.
func newTSEFiliadosRouter(h *handlers.TSEExtrasHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/v1/eleicoes/filiados", h.GetFiliados)
	return r
}

func TestTSEExtrasHandler_GetFiliados_OK(t *testing.T) {
	// CSV must include sg_uf column so the handler can filter by UF in-memory.
	csvContent := "SG_UF;NM_PARTIDO;NM_CANDIDATO\n" +
		"SP;PARTIDO A;JOSE DA SILVA\n" +
		"SP;PARTIDO B;MARIA SOUZA\n" +
		"SP;PARTIDO C;PEDRO SANTOS\n" +
		"RJ;PARTIDO A;OUTRO CANDIDATO\n" // should be filtered out
	zipBytes := buildTSEZip(t, "perfil_filiacao_partidaria.csv", csvContent)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write(zipBytes)
	}))
	defer srv.Close()

	h := mockTSEFiliadosHandler(t, srv)
	router := newTSEFiliadosRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/filiados?uf=SP&n=3", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "tse_filiados" {
		t.Errorf("Source = %q, want tse_filiados", resp.Source)
	}
	filiados, ok := resp.Data["filiados"].([]any)
	if !ok || len(filiados) == 0 {
		t.Errorf("expected non-empty filiados slice, got %T: %v", resp.Data["filiados"], resp.Data["filiados"])
	}
	if len(filiados) != 3 {
		t.Errorf("expected 3 filiados, got %d", len(filiados))
	}
	total, _ := resp.Data["total"].(float64)
	if int(total) != 3 {
		t.Errorf("expected total=3, got %v", resp.Data["total"])
	}
	uf, _ := resp.Data["uf"].(string)
	if uf != "SP" {
		t.Errorf("expected uf=SP, got %q", uf)
	}
}

func TestTSEExtrasHandler_GetFiliados_MissingUF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := mockTSEFiliadosHandler(t, srv)
	router := newTSEFiliadosRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/filiados", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTSEExtrasHandler_GetFiliados_InvalidUF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := mockTSEFiliadosHandler(t, srv)
	router := newTSEFiliadosRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/filiados?uf=XX", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestTSEExtras_GetBens_NLimit verifies the n param limits rows returned.
func TestTSEExtras_GetBens_NLimit(t *testing.T) {
	// CSV with 5 rows
	csvContent := "SQ_CANDIDATO;NM_CANDIDATO;VR_BEM_CANDIDATO\n" +
		"1;A;100\n2;B;200\n3;C;300\n4;D;400\n5;E;500\n"
	zipBytes := buildTSEZip(t, "consulta_bem_candidato_2024_BRASIL.csv", csvContent)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		w.Write(zipBytes)
	}))
	defer srv.Close()

	h := mockTSEExtras(t, srv)
	router := newTSEExtrasRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/eleicoes/bens?n=3", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	bens, ok := resp.Data["bens"].([]any)
	if !ok {
		t.Fatalf("expected bens to be []any, got %T", resp.Data["bens"])
	}
	if len(bens) != 3 {
		t.Errorf("expected 3 bens (n=3), got %d", len(bens))
	}
	total, _ := resp.Data["total"].(float64) // JSON numbers decode as float64
	if int(total) != 3 {
		t.Errorf("expected total=3, got %v", resp.Data["total"])
	}
}
