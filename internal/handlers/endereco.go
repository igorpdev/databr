package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

var nonDigit = regexp.MustCompile(`\D`)

// EnderecoHandler handles requests for /v1/endereco/*.
type EnderecoHandler struct {
	httpClient *http.Client
}

// NewEnderecoHandler creates a new EnderecoHandler with a default HTTP client.
func NewEnderecoHandler() *EnderecoHandler {
	return &EnderecoHandler{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// NewEnderecoHandlerWithClient creates a new EnderecoHandler using the provided HTTP client.
// Useful for testing with a custom transport that redirects to a mock server.
func NewEnderecoHandlerWithClient(client *http.Client) *EnderecoHandler {
	return &EnderecoHandler{httpClient: client}
}

// GetEndereco handles GET /v1/endereco/{cep}.
// Proxies to viacep.com.br/ws/{cep}/json/.
// CEP can be "01310100" or "01310-100" — the hyphen is stripped before lookup.
func (h *EnderecoHandler) GetEndereco(w http.ResponseWriter, r *http.Request) {
	rawCEP := chi.URLParam(r, "cep")
	cep := nonDigit.ReplaceAllString(rawCEP, "")

	if len(cep) != 8 {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("CEP deve ter exatamente 8 dígitos, recebido: %q", rawCEP))
		return
	}

	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)
	resp, err := h.httpClient.Get(url)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao consultar ViaCEP: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		jsonError(w, http.StatusBadGateway, fmt.Sprintf("ViaCEP retornou status %d", resp.StatusCode))
		return
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		jsonError(w, http.StatusBadGateway, "Erro ao decodificar resposta ViaCEP: "+err.Error())
		return
	}

	// ViaCEP returns {"erro": "true"} (as string) for CEPs not found.
	if erroVal, ok := raw["erro"]; ok {
		erroStr := fmt.Sprintf("%v", erroVal)
		if strings.EqualFold(erroStr, "true") {
			jsonError(w, http.StatusNotFound, "CEP não encontrado: "+cep)
			return
		}
	}

	respond(w, r, domain.APIResponse{
		Source:   "viacep",
		CostUSDC: "0.001",
		Data:     raw,
	})
}
