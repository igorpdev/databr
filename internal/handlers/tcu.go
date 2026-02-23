package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// TCUHandler handles requests for /v1/tcu/*.
type TCUHandler struct {
	httpClient *http.Client
}

// NewTCUHandler creates a new TCUHandler with a default HTTP client (15s timeout).
func NewTCUHandler() *TCUHandler {
	return &TCUHandler{httpClient: &http.Client{Timeout: 15 * time.Second}}
}

// NewTCUHandlerWithClient creates a new TCUHandler using the provided HTTP client.
func NewTCUHandlerWithClient(client *http.Client) *TCUHandler {
	return &TCUHandler{httpClient: client}
}

// GetAcordaos handles GET /v1/tcu/acordaos?inicio=0&quantidade=20.
func (h *TCUHandler) GetAcordaos(w http.ResponseWriter, r *http.Request) {
	inicio := r.URL.Query().Get("inicio")
	if inicio == "" {
		inicio = "0"
	}
	quantidade := r.URL.Query().Get("quantidade")
	if quantidade == "" {
		quantidade = "20"
	}
	if _, err := strconv.Atoi(inicio); err != nil {
		jsonError(w, http.StatusBadRequest, "param 'inicio' deve ser numérico")
		return
	}
	if _, err := strconv.Atoi(quantidade); err != nil {
		jsonError(w, http.StatusBadRequest, "param 'quantidade' deve ser numérico")
		return
	}

	params := url.Values{}
	params.Set("inicio", inicio)
	params.Set("quantidade", quantidade)
	upURL := "https://dados-abertos.apps.tcu.gov.br/api/acordao/recupera-acordaos?" + params.Encode()

	var dados any
	if _, err := fetchJSON(r.Context(), h.httpClient, upURL, nil, &dados); err != nil {
		gatewayError(w, "tcu_acordaos", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "tcu_acordaos",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"acordaos": dados},
	})
}

// GetCertidao handles GET /v1/tcu/certidao/{cnpj}.
func (h *TCUHandler) GetCertidao(w http.ResponseWriter, r *http.Request) {
	cnpj := chi.URLParam(r, "cnpj")
	cnpj = normalizeCNPJdigits(cnpj)
	if len(cnpj) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos")
		return
	}

	upURL := fmt.Sprintf("https://certidoes-apf.apps.tcu.gov.br/api/rest/publico/certidoes/%s", cnpj)

	var dados any
	status, err := fetchJSON(r.Context(), h.httpClient, upURL, nil, &dados)
	if err != nil {
		if status == http.StatusNotFound {
			jsonError(w, http.StatusNotFound, "CNPJ não encontrado no TCU: "+cnpj)
			return
		}
		gatewayError(w, "tcu_certidao", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "tcu_certidao",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"cnpj": cnpj, "certidao": dados},
	})
}

// GetInabilitados handles GET /v1/tcu/inabilitados.
func (h *TCUHandler) GetInabilitados(w http.ResponseWriter, r *http.Request) {
	upURL := "https://contas.tcu.gov.br/ords/condenacao/consulta/inabilitados"

	var dados any
	if _, err := fetchJSON(r.Context(), h.httpClient, upURL, nil, &dados); err != nil {
		gatewayError(w, "tcu_inabilitados", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "tcu_inabilitados",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"inabilitados": dados},
	})
}

// GetInabilitadoByCPF handles GET /v1/tcu/inabilitados/{cpf}.
func (h *TCUHandler) GetInabilitadoByCPF(w http.ResponseWriter, r *http.Request) {
	cpf := chi.URLParam(r, "cpf")
	clean := reDigits.ReplaceAllString(cpf, "")
	if len(clean) != 11 {
		jsonError(w, http.StatusBadRequest, "CPF inválido — deve ter 11 dígitos")
		return
	}

	upURL := "https://contas.tcu.gov.br/ords/condenacao/consulta/inabilitados/" + clean

	var dados any
	if _, err := fetchJSON(r.Context(), h.httpClient, upURL, nil, &dados); err != nil {
		gatewayError(w, "tcu_inabilitados", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "tcu_inabilitados",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"cpf": clean, "inabilitado": dados},
	})
}

// GetContratos handles GET /v1/tcu/contratos.
func (h *TCUHandler) GetContratos(w http.ResponseWriter, r *http.Request) {
	upURL := "https://contas.tcu.gov.br/contrata2RS/api/publico/termos-contratuais"

	var dados any
	if _, err := fetchJSON(r.Context(), h.httpClient, upURL, nil, &dados); err != nil {
		gatewayError(w, "tcu_contratos", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "tcu_contratos",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"contratos": dados},
	})
}
