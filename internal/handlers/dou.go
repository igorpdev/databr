package handlers

import (
	"context"
	"net/http"

	"github.com/databr/api/internal/collectors/dou"
	"github.com/databr/api/internal/domain"
)

// QDSearcher is the interface for on-demand Querido Diário search.
type QDSearcher interface {
	Search(ctx context.Context, params dou.SearchParams) ([]domain.SourceRecord, error)
}

// DOUHandler handles /v1/dou/* requests.
type DOUHandler struct {
	searcher QDSearcher
}

// NewDOUHandler creates a DOU handler backed by Querido Diário.
func NewDOUHandler(searcher QDSearcher) *DOUHandler {
	return &DOUHandler{searcher: searcher}
}

// GetBusca handles GET /v1/dou/busca?q=...&uf=SP&desde=2026-01-01&ate=2026-02-01
func (h *DOUHandler) GetBusca(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonError(w, http.StatusBadRequest, "query param 'q' is required")
		return
	}

	params := dou.SearchParams{
		Query: q,
		UF:    r.URL.Query().Get("uf"),
		Since: r.URL.Query().Get("desde"),
		Until: r.URL.Query().Get("ate"),
		Size:  20,
	}

	records, err := h.searcher.Search(r.Context(), params)
	if err != nil {
		gatewayError(w, "dou", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No results found for: "+q)
		return
	}

	items := make([]map[string]any, len(records))
	for i, rec := range records {
		items[i] = rec.Data
	}
	respond(w, r, domain.APIResponse{
		Source:    "querido_diario",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  "0.003",
		Data:      map[string]any{"resultados": items, "total": len(items), "query": q},
	})
}

// GetDiarios handles GET /v1/diarios/busca.
// Query params: q (required), municipio_ibge (IBGE municipality code), desde (YYYY-MM-DD), ate (YYYY-MM-DD).
func (h *DOUHandler) GetDiarios(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonError(w, http.StatusBadRequest, "query param 'q' is required")
		return
	}

	params := dou.SearchParams{
		Query:       q,
		TerritoryID: r.URL.Query().Get("municipio_ibge"),
		Since:       r.URL.Query().Get("desde"),
		Until:       r.URL.Query().Get("ate"),
		Size:        20,
	}

	records, err := h.searcher.Search(r.Context(), params)
	if err != nil {
		gatewayError(w, "dou", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No results found for: "+q)
		return
	}

	items := make([]map[string]any, len(records))
	for i, rec := range records {
		items[i] = rec.Data
	}
	respond(w, r, domain.APIResponse{
		Source:    "querido_diario",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  "0.003",
		Data: map[string]any{
			"resultados":    items,
			"total":         len(items),
			"query":         q,
			"municipio_ibge": params.TerritoryID,
		},
	})
}
