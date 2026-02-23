package handlers

import (
	"net/http"

	"github.com/databr/api/internal/domain"
)

// TransparenciaHandler handles /v1/transparencia/* and /v1/eleicoes/* endpoints.
type TransparenciaHandler struct {
	store SourceStore
}

// NewTransparenciaHandler creates a handler for PNCP and TSE data.
func NewTransparenciaHandler(store SourceStore) *TransparenciaHandler {
	return &TransparenciaHandler{store: store}
}

// GetLicitacoes handles GET /v1/transparencia/licitacoes.
// Returns the 100 most recent PNCP procurement records.
func (h *TransparenciaHandler) GetLicitacoes(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "pncp_licitacoes")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No procurement records available yet")
		return
	}
	items := make([]map[string]any, len(records))
	for i, rec := range records {
		items[i] = rec.Data
	}
	respond(w, r, domain.APIResponse{
		Source:    "pncp_licitacoes",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  "0.001",
		Data:      map[string]any{"licitacoes": items, "total": len(items)},
	})
}

// GetCandidatos handles GET /v1/eleicoes/candidatos.
// Returns TSE candidate records. Supports optional ?uf=SP filter.
func (h *TransparenciaHandler) GetCandidatos(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "tse_candidatos")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Optional filter by UF
	if uf := r.URL.Query().Get("uf"); uf != "" {
		var filtered []domain.SourceRecord
		for _, rec := range records {
			if sgUF, _ := rec.Data["sg_uf"].(string); sgUF == uf {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No candidate records available yet")
		return
	}
	items := make([]map[string]any, len(records))
	for i, rec := range records {
		items[i] = rec.Data
	}
	respond(w, r, domain.APIResponse{
		Source:    "tse_candidatos",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  "0.001",
		Data:      map[string]any{"candidatos": items, "total": len(items)},
	})
}
