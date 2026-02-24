package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/databr/api/internal/collectors/juridico"
	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// DataJudSearcher searches judicial processes by CPF/CNPJ or process number.
type DataJudSearcher interface {
	Search(ctx context.Context, documento string) ([]domain.SourceRecord, error)
	SearchByNumber(ctx context.Context, numero string) ([]domain.SourceRecord, error)
}

// JudicialHandler handles /v1/judicial/* requests.
type JudicialHandler struct {
	searcher DataJudSearcher
}

// NewJudicialHandler creates a judicial handler.
func NewJudicialHandler(searcher DataJudSearcher) *JudicialHandler {
	return &JudicialHandler{searcher: searcher}
}

// GetProcessos handles GET /v1/judicial/processos/{doc}.
// {doc} can be a CPF (11 digits) or CNPJ (14 digits), with or without formatting.
func (h *JudicialHandler) GetProcessos(w http.ResponseWriter, r *http.Request) {
	doc := chi.URLParam(r, "doc")
	if doc == "" {
		jsonError(w, http.StatusBadRequest, "documento (CPF ou CNPJ) is required")
		return
	}
	if !isValidCPFOrCNPJ(doc) {
		jsonError(w, http.StatusBadRequest, "documento inválido — deve ser CPF (11 dígitos) ou CNPJ (14 dígitos)")
		return
	}

	records, err := h.searcher.Search(r.Context(), doc)
	if err != nil {
		gatewayError(w, "judicial", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No processes found for document "+doc)
		return
	}

	items := make([]map[string]any, len(records))
	for i, rec := range records {
		items[i] = rec.Data
	}
	respond(w, r, domain.APIResponse{
		Source:    "datajud_cnj",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"processos": items, "total": len(items), "documento": doc},
	})
}

// GetProcesso handles GET /v1/judicial/processo/{numero}.
// {numero} is a CNJ unified process number (e.g. 0000832-35.2018.4.01.3202).
func (h *JudicialHandler) GetProcesso(w http.ResponseWriter, r *http.Request) {
	numero := chi.URLParam(r, "numero")
	if numero == "" {
		jsonError(w, http.StatusBadRequest, "número do processo is required")
		return
	}
	// Minimal format validation: CNJ numbers contain hyphens and dots
	if !strings.Contains(numero, "-") || !strings.Contains(numero, ".") {
		jsonError(w, http.StatusBadRequest, "formato inválido — use o número CNJ unificado (ex: 0000832-35.2018.4.01.3202)")
		return
	}

	records, err := h.searcher.SearchByNumber(r.Context(), numero)
	if err != nil {
		var parseErr *juridico.ParseError
		if errors.As(err, &parseErr) {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		gatewayError(w, "judicial", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "Processo não encontrado: "+numero)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "datajud_cnj",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      records[0].Data,
	})
}
