package x402

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// routeMetaEntry holds human-readable metadata for a single API endpoint.
type routeMetaEntry struct {
	description string
	mimeType    string
}

// routeMeta maps Chi route patterns to metadata surfaced in the x402 Bazaar index.
// Keys must match the full pattern returned by chi.RouteContext.RoutePattern(),
// including any /v1 prefix from r.Route("/v1", ...).
var routeMeta = map[string]routeMetaEntry{
	"/v1/empresas/{cnpj}":            {"Dados completos de empresa por CNPJ", "application/json"},
	"/v1/empresas/{cnpj}/compliance": {"Consulta de conformidade e compliance de empresa", "application/json"},
	"/v1/bcb/selic":                  {"Taxa Selic do Banco Central", "application/json"},
	"/v1/bcb/cambio/{moeda}":         {"Cotação PTAX do Banco Central", "application/json"},
	"/v1/bcb/pix/estatisticas":       {"Estatísticas do sistema PIX", "application/json"},
	"/v1/bcb/credito":                {"Dados de crédito do Banco Central", "application/json"},
	"/v1/bcb/reservas":               {"Reservas internacionais do Brasil", "application/json"},
	"/v1/economia/ipca":              {"Índice de inflação IPCA (IBGE)", "application/json"},
	"/v1/economia/pib":               {"Produto Interno Bruto (IBGE)", "application/json"},
	"/v1/mercado/acoes/{ticker}":     {"Cotação histórica de ações da B3", "application/json"},
	"/v1/mercado/fundos/{cnpj}":      {"Dados de fundos de investimento (CVM)", "application/json"},
	"/v1/compliance/{cnpj}":          {"Verificação completa de compliance por CNPJ", "application/json"},
	"/v1/transparencia/licitacoes":   {"Licitações públicas (PNCP)", "application/json"},
	"/v1/eleicoes/candidatos":        {"Dados de candidatos eleitorais (TSE)", "application/json"},
	"/v1/tesouro/rreo":                  {"Relatório de Execução Orçamentária (Tesouro Nacional)", "application/json"},
	"/v1/dou/busca":                    {"Busca no Diário Oficial da União", "application/json"},
	"/v1/judicial/processos/{doc}":     {"Processos judiciais por CPF/CNPJ (DataJud CNJ)", "application/json"},
	"/v1/economia/focus":               {"Expectativas de mercado do Relatório Focus (BCB)", "application/json"},
	"/v1/saude/medicamentos/{registro}": {"Medicamentos registrados na ANVISA", "application/json"},
	"/v1/energia/tarifas":              {"Tarifas de energia elétrica homologadas pela ANEEL", "application/json"},
	"/v1/ambiental/desmatamento":       {"Alertas de desmatamento em tempo real (INPE DETER)", "application/json"},
	"/v1/ambiental/prodes":             {"Desmatamento anual consolidado (INPE PRODES)", "application/json"},
}

// BazaarMiddleware intercepts HTTP 402 responses and injects an `extensions.bazaar`
// field into the JSON body, making the endpoint discoverable by the x402 Bazaar index.
//
// Register BEFORE the x402 payment middleware in the Chi middleware chain so that
// this wrapper sees the 402 emitted by the payment gate:
//
//	r.Use(x402pkg.BazaarMiddleware())
//	r.Use(optionalX402(cfg, "0.001"))
func BazaarMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bw := &bazaarWriter{ResponseWriter: w}
			next.ServeHTTP(bw, r)

			var pattern string
			if rc := chi.RouteContext(r.Context()); rc != nil {
				pattern = rc.RoutePattern()
			}
			bw.finalize(pattern)
		})
	}
}

// bazaarWriter wraps http.ResponseWriter to buffer 402 responses for extension injection.
// Non-402 responses are written through immediately without buffering.
type bazaarWriter struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

// WriteHeader delays writing 402 status so we can modify the body first.
// All other status codes pass through immediately.
func (bw *bazaarWriter) WriteHeader(code int) {
	bw.status = code
	if code != http.StatusPaymentRequired {
		bw.ResponseWriter.WriteHeader(code)
	}
}

// Write buffers the body when a 402 has been signalled; otherwise passes through.
func (bw *bazaarWriter) Write(b []byte) (int, error) {
	if bw.status == http.StatusPaymentRequired {
		return bw.buf.Write(b)
	}
	return bw.ResponseWriter.Write(b)
}

// finalize writes the (possibly modified) buffered response.
// For 402 responses it injects the bazaar discovery extension into the JSON body.
// For all other responses it is a no-op (already written through).
func (bw *bazaarWriter) finalize(pattern string) {
	if bw.status != http.StatusPaymentRequired {
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(bw.buf.Bytes(), &body); err != nil {
		// Body is not valid JSON — write original response unchanged.
		bw.ResponseWriter.WriteHeader(http.StatusPaymentRequired)
		bw.ResponseWriter.Write(bw.buf.Bytes()) //nolint:errcheck
		return
	}

	meta, ok := routeMeta[pattern]
	if !ok {
		meta = routeMetaEntry{"DataBR — dados públicos brasileiros", "application/json"}
	}

	body["extensions"] = map[string]interface{}{
		"bazaar": map[string]interface{}{
			"discoverable":   true,
			"method":         "GET",
			"description":    meta.description,
			"outputMimeType": meta.mimeType,
		},
	}

	modified, err := json.Marshal(body)
	if err != nil {
		bw.ResponseWriter.WriteHeader(http.StatusPaymentRequired)
		bw.ResponseWriter.Write(bw.buf.Bytes()) //nolint:errcheck
		return
	}

	bw.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(modified)))
	bw.ResponseWriter.WriteHeader(http.StatusPaymentRequired)
	bw.ResponseWriter.Write(modified) //nolint:errcheck
}
