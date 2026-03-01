package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

const transparenciaAPIBase = "https://api.portaldatransparencia.gov.br/api-de-dados"

// TransparenciaFetcher retrieves on-demand CGU Portal da Transparência data.
type TransparenciaFetcher interface {
	// FetchContratos returns contracts for a government agency (orgao = SIAFI code, e.g. "26000").
	// cnpjFornecedor is optional — pass "" to skip supplier filtering.
	FetchContratos(ctx context.Context, orgao, cnpjFornecedor string) ([]domain.SourceRecord, error)
	FetchServidores(ctx context.Context, orgao string) ([]domain.SourceRecord, error)
	FetchBolsaFamilia(ctx context.Context, municipioIBGE, mesAno string) ([]domain.SourceRecord, error)
	FetchCartoes(ctx context.Context, orgao, de, ate string) ([]domain.SourceRecord, error)
}

// TransparenciaFederalHandler handles /v1/transparencia/contratos,
// /v1/transparencia/servidores, and /v1/transparencia/beneficios.
type TransparenciaFederalHandler struct {
	fetcher    TransparenciaFetcher
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// NewTransparenciaFederalHandler creates a TransparenciaFederalHandler.
func NewTransparenciaFederalHandler(f TransparenciaFetcher) *TransparenciaFederalHandler {
	apiKey := os.Getenv("TRANSPARENCIA_API_KEY")
	if apiKey == "" {
		slog.Warn("TRANSPARENCIA_API_KEY not set — transparencia endpoints will fail")
	}
	return &TransparenciaFederalHandler{
		fetcher:    f,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		apiKey:     apiKey,
		baseURL:    transparenciaAPIBase,
	}
}

// NewTransparenciaFederalHandlerWithClient creates a TransparenciaFederalHandler with a custom HTTP client and API key.
func NewTransparenciaFederalHandlerWithClient(f TransparenciaFetcher, client *http.Client, apiKey string) *TransparenciaFederalHandler {
	return &TransparenciaFederalHandler{fetcher: f, httpClient: client, apiKey: apiKey, baseURL: transparenciaAPIBase}
}

// NewTransparenciaFederalHandlerWithBaseURL is for testing — allows custom base URL.
func NewTransparenciaFederalHandlerWithBaseURL(f TransparenciaFetcher, client *http.Client, apiKey, baseURL string) *TransparenciaFederalHandler {
	return &TransparenciaFederalHandler{
		fetcher:    f,
		httpClient: client,
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// reDigits is defined in helpers.go

// normalizeCNPJdigits strips all non-digit characters.
func normalizeCNPJdigits(s string) string {
	return reDigits.ReplaceAllString(s, "")
}

// prevMonthYYYYMM returns the previous calendar month in YYYYMM format.
func prevMonthYYYYMM() string {
	t := time.Now().UTC().AddDate(0, -1, 0)
	return t.Format("200601")
}

// GetContratos handles GET /v1/transparencia/contratos?orgao={codigoOrgao}[&cnpj={cnpjFornecedor}]
// orgao is the mandatory SIAFI agency code (e.g. "26000" for MEC).
// cnpj is optional — when provided, filters contracts by supplier CNPJ.
func (h *TransparenciaFederalHandler) GetContratos(w http.ResponseWriter, r *http.Request) {
	orgao := r.URL.Query().Get("orgao")
	if orgao == "" {
		jsonError(w, http.StatusBadRequest, "query param 'orgao' is required (SIAFI agency code, e.g. '26000')")
		return
	}

	cnpj := ""
	if raw := r.URL.Query().Get("cnpj"); raw != "" {
		cnpj = normalizeCNPJdigits(raw)
	}

	records, err := h.fetcher.FetchContratos(r.Context(), orgao, cnpj)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No contracts found for orgao "+orgao)
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// GetServidores handles GET /v1/transparencia/servidores?orgao=
func (h *TransparenciaFederalHandler) GetServidores(w http.ResponseWriter, r *http.Request) {
	orgao := r.URL.Query().Get("orgao")
	if orgao == "" {
		jsonError(w, http.StatusBadRequest, "query param 'orgao' is required")
		return
	}

	records, err := h.fetcher.FetchServidores(r.Context(), orgao)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No servers found for organ "+orgao)
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// GetBolsaFamilia handles GET /v1/transparencia/beneficios?municipio_ibge=&mes=
func (h *TransparenciaFederalHandler) GetBolsaFamilia(w http.ResponseWriter, r *http.Request) {
	municipio := r.URL.Query().Get("municipio_ibge")
	if municipio == "" {
		jsonError(w, http.StatusBadRequest, "query param 'municipio_ibge' is required")
		return
	}

	mes := r.URL.Query().Get("mes")
	if mes == "" {
		mes = prevMonthYYYYMM()
	}

	records, err := h.fetcher.FetchBolsaFamilia(r.Context(), municipio, mes)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No Bolsa Família data found for municipio "+municipio+" mes "+mes)
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// GetCartoes handles GET /v1/transparencia/cartoes?orgao=&de=&ate=
// orgao is the mandatory SIAFI agency code (e.g. "26000" for MEC).
// de and ate are optional dates in YYYY-MM-DD format (default: last 30 days).
func (h *TransparenciaFederalHandler) GetCartoes(w http.ResponseWriter, r *http.Request) {
	orgao := r.URL.Query().Get("orgao")
	if orgao == "" {
		jsonError(w, http.StatusBadRequest, "query param 'orgao' is required (SIAFI agency code, e.g. '26000')")
		return
	}

	de := r.URL.Query().Get("de")
	ate := r.URL.Query().Get("ate")
	if de == "" {
		de = time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if ate == "" {
		ate = time.Now().UTC().Format("2006-01-02")
	}

	records, err := h.fetcher.FetchCartoes(r.Context(), orgao, de, ate)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No cartões transactions found for orgao "+orgao)
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// GetCEAF handles GET /v1/transparencia/ceaf/{cnpj}.
// Returns CEAF (Cadastro de Entidades sem Fins Lucrativos) data for a CNPJ.
// Requires TRANSPARENCIA_API_KEY env variable.
func (h *TransparenciaFederalHandler) GetCEAF(w http.ResponseWriter, r *http.Request) {
	cnpj := chi.URLParam(r, "cnpj")
	cnpj = normalizeCNPJdigits(cnpj)
	if len(cnpj) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos")
		return
	}

	upURL := fmt.Sprintf(
		"https://api.portaldatransparencia.gov.br/api-de-dados/ceaf?CNPJ=%s&pagina=1",
		cnpj,
	)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_federal", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		jsonError(w, http.StatusNotFound, "CNPJ não encontrado no CEAF: "+cnpj)
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência CEAF", resp.StatusCode, body))
		return
	}

	var dados any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_ceaf",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"cnpj": cnpj, "ceaf": dados},
	})
}

// GetEmendas handles GET /v1/transparencia/emendas?ano=2024&n=20.
// Returns parliamentary budget amendments from Portal da Transparência.
// Optional: ano (default current year), n (default 20, max 100).
// Requires TRANSPARENCIA_API_KEY env variable.
func (h *TransparenciaFederalHandler) GetEmendas(w http.ResponseWriter, r *http.Request) {
	ano := r.URL.Query().Get("ano")
	if ano == "" {
		ano = strconv.Itoa(time.Now().UTC().Year())
	}
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	upURL := fmt.Sprintf(
		"https://api.portaldatransparencia.gov.br/api-de-dados/emendas?pagina=1&quantidade=%d&ano=%s",
		n, ano,
	)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_federal", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência Emendas", resp.StatusCode, body))
		return
	}

	var dados []any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_emendas",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"emendas": dados, "total": len(dados), "ano": ano},
	})
}

// GetObras handles GET /v1/transparencia/obras?n=20.
// Returns federal government functional real estate records from Portal da Transparência.
// Uses the /imoveis endpoint (functional property registry) as the works/obras data source.
// Optional: n (default 20, max 100).
// Requires TRANSPARENCIA_API_KEY env variable.
func (h *TransparenciaFederalHandler) GetObras(w http.ResponseWriter, r *http.Request) {
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	upURL := fmt.Sprintf(
		"https://api.portaldatransparencia.gov.br/api-de-dados/imoveis?pagina=1&quantidade=%d",
		n,
	)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_federal", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência Obras", resp.StatusCode, body))
		return
	}

	var dados []any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_obras",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"obras": dados, "total": len(dados)},
	})
}

// GetTransferencias handles GET /v1/transparencia/transferencias?orgao=26000&n=20.
// Returns federal government convenios/transfers for the given agency (SIAFI code).
// Uses the /convenios endpoint — the API requires at least one filter (orgao is mandatory here).
// Required: orgao (SIAFI agency code, e.g. "26000" for MEC).
// Optional: municipio_ibge (IBGE municipality code), n (default 20, max 100).
// Requires TRANSPARENCIA_API_KEY env variable.
func (h *TransparenciaFederalHandler) GetTransferencias(w http.ResponseWriter, r *http.Request) {
	orgao := r.URL.Query().Get("orgao")
	if orgao == "" {
		jsonError(w, http.StatusBadRequest, "query param 'orgao' é obrigatório (código SIAFI, ex: '26000' para MEC)")
		return
	}
	municipio := r.URL.Query().Get("municipio_ibge")
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	upURL := fmt.Sprintf(
		"https://api.portaldatransparencia.gov.br/api-de-dados/convenios?pagina=1&quantidade=%d&codigoOrgao=%s",
		n, orgao,
	)
	if municipio != "" {
		upURL += "&codigoIBGE=" + municipio
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_federal", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência Transferencias", resp.StatusCode, body))
		return
	}

	var dados []any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_transferencias",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"transferencias": dados, "total": len(dados), "orgao": orgao, "municipio_ibge": municipio},
	})
}

// GetPensionistas handles GET /v1/transparencia/pensionistas?orgao=26000&n=20.
// Returns federal government civil servants (tipoServidor=1) by agency from Portal da Transparência.
// Uses the /servidores endpoint filtered by orgaoServidorLotacao and tipoServidor=1 (civil).
// Required: orgao (SIAPI agency code, e.g. "26000" for MEC).
// Optional: n (default 20, max 100).
// Requires TRANSPARENCIA_API_KEY env variable.
func (h *TransparenciaFederalHandler) GetPensionistas(w http.ResponseWriter, r *http.Request) {
	orgao := r.URL.Query().Get("orgao")
	if orgao == "" {
		jsonError(w, http.StatusBadRequest, "query param 'orgao' is required (SIAFI agency code, e.g. '26000')")
		return
	}
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	upURL := fmt.Sprintf(
		"https://api.portaldatransparencia.gov.br/api-de-dados/servidores?orgaoServidorLotacao=%s&tipoServidor=1&pagina=1&quantidade=%d",
		orgao, n,
	)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_federal", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência Pensionistas", resp.StatusCode, body))
		return
	}

	var dados []any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_pensionistas",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"pensionistas": dados, "total": len(dados), "orgao": orgao},
	})
}

// GetViagens handles GET /v1/transparencia/viagens.
// Returns government travel records from Portal da Transparência.
// Required query param: orgao (SIAFI agency code, e.g. "26000" for MEC).
// Optional: de (YYYY-MM-DD, default last 30 days), ate (YYYY-MM-DD), n (default 20, max 100).
// Requires TRANSPARENCIA_API_KEY env variable.
func (h *TransparenciaFederalHandler) GetViagens(w http.ResponseWriter, r *http.Request) {
	orgao := r.URL.Query().Get("orgao")
	if orgao == "" {
		jsonError(w, http.StatusBadRequest, "query param 'orgao' is required (SIAFI agency code, e.g. '26000')")
		return
	}
	de := r.URL.Query().Get("de")
	ate := r.URL.Query().Get("ate")
	if de == "" {
		de = time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if ate == "" {
		ate = time.Now().UTC().Format("2006-01-02")
	}
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	upURL := fmt.Sprintf(
		"https://api.portaldatransparencia.gov.br/api-de-dados/viagens?pagina=1&quantidade=%d&codigoOrgao=%s&dataIdaDe=%s&dataIdaAte=%s&dataRetornoDe=%s&dataRetornoAte=%s",
		n, orgao, de, ate, de, ate,
	)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_federal", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência Viagens", resp.StatusCode, body))
		return
	}

	var dados []any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_federal", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_viagens",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"viagens": dados, "total": len(dados), "de": de, "ate": ate},
	})
}

// GetPGFN handles GET /v1/transparencia/pgfn/{cnpj}
// Returns PGFN dívida ativa records for a given CNPJ.
func (h *TransparenciaFederalHandler) GetPGFN(w http.ResponseWriter, r *http.Request) {
	cnpj := normalizeCNPJdigits(chi.URLParam(r, "cnpj"))
	if len(cnpj) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos")
		return
	}

	upURL := fmt.Sprintf("%s/pgfn/consultaReceitaCadastro?cnpj=%s&pagina=1", h.baseURL, cnpj)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_pgfn", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_pgfn", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		jsonError(w, http.StatusNotFound, "CNPJ não encontrado no PGFN: "+cnpj)
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência PGFN", resp.StatusCode, body))
		return
	}

	var dados any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_pgfn", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_pgfn",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"cnpj": cnpj, "pgfn": dados},
	})
}

// GetPEP handles GET /v1/transparencia/pep/{cpf}
// Returns PEP (Pessoa Politicamente Exposta) records for a given CPF.
func (h *TransparenciaFederalHandler) GetPEP(w http.ResponseWriter, r *http.Request) {
	cpf := reDigits.ReplaceAllString(chi.URLParam(r, "cpf"), "")
	if len(cpf) != 11 {
		jsonError(w, http.StatusBadRequest, "CPF inválido — deve ter 11 dígitos")
		return
	}

	upURL := fmt.Sprintf("%s/pep?cpf=%s&pagina=1", h.baseURL, cpf)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_pep", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_pep", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência PEP", resp.StatusCode, body))
		return
	}

	var dados any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_pep", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_pep",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"cpf": cpf, "pep": dados},
	})
}

// GetLeniencias handles GET /v1/transparencia/leniencias/{cnpj}
// Returns CGU leniency agreements for a given CNPJ.
func (h *TransparenciaFederalHandler) GetLeniencias(w http.ResponseWriter, r *http.Request) {
	cnpj := normalizeCNPJdigits(chi.URLParam(r, "cnpj"))
	if len(cnpj) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos")
		return
	}

	upURL := fmt.Sprintf("%s/leniencias?cnpj=%s&pagina=1", h.baseURL, cnpj)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_leniencias", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_leniencias", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência Leniências", resp.StatusCode, body))
		return
	}

	var dados any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_leniencias", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_leniencias",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"cnpj": cnpj, "leniencias": dados},
	})
}

// GetRenuncias handles GET /v1/transparencia/renuncias?ano=2024&n=20
// Returns fiscal tax waivers for a given year.
func (h *TransparenciaFederalHandler) GetRenuncias(w http.ResponseWriter, r *http.Request) {
	ano := r.URL.Query().Get("ano")
	if ano == "" {
		ano = strconv.Itoa(time.Now().UTC().Year())
	}
	n := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}

	upURL := fmt.Sprintf("%s/renuncias-fiscais?exercicio=%s&pagina=1&quantidade=%d", h.baseURL, ano, n)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upURL, nil)
	if err != nil {
		internalError(w, "transparencia_renuncias", err)
		return
	}
	req.Header.Set("chave-api-dados", h.apiKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		gatewayError(w, "transparencia_renuncias", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		jsonError(w, http.StatusServiceUnavailable, "TRANSPARENCIA_API_KEY não configurada ou inválida")
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body)
		jsonError(w, http.StatusBadGateway, logUpstreamError("Portal Transparência Renúncias", resp.StatusCode, body))
		return
	}

	var dados []any
	if err := json.NewDecoder(resp.Body).Decode(&dados); err != nil {
		gatewayError(w, "transparencia_renuncias", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:   "cgu_renuncias",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data:     map[string]any{"renuncias": dados, "total": len(dados), "ano": ano},
	})
}
