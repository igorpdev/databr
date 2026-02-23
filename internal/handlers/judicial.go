package handlers

import (
	"context"
	"net/http"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// DataJudSearcher searches judicial processes by CPF/CNPJ.
type DataJudSearcher interface {
	Search(ctx context.Context, documento string) ([]domain.SourceRecord, error)
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
		jsonError(w, http.StatusBadGateway, err.Error())
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
		CostUSDC:  "0.010",
		Data:      map[string]any{"processos": items, "total": len(items), "documento": doc},
	})
}
