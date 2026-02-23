package handlers

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/databr/api/internal/domain"
)

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
	fetcher TransparenciaFetcher
}

// NewTransparenciaFederalHandler creates a TransparenciaFederalHandler.
func NewTransparenciaFederalHandler(f TransparenciaFetcher) *TransparenciaFederalHandler {
	return &TransparenciaFederalHandler{fetcher: f}
}

var reDigits = regexp.MustCompile(`\D`)

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
		jsonError(w, http.StatusBadGateway, err.Error())
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
		CostUSDC:  "0.001",
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
		jsonError(w, http.StatusBadGateway, err.Error())
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
		CostUSDC:  "0.001",
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
		jsonError(w, http.StatusBadGateway, err.Error())
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
		CostUSDC:  "0.001",
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
		jsonError(w, http.StatusBadGateway, err.Error())
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
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}
