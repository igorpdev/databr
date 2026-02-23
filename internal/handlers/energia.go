package handlers

import (
	"net/http"
	"strings"

	"github.com/databr/api/internal/domain"
)

// EnergiaHandler handles requests for /v1/energia/*.
type EnergiaHandler struct {
	store SourceStore
}

// NewEnergiaHandler creates an Energia handler backed by the provided SourceStore.
func NewEnergiaHandler(store SourceStore) *EnergiaHandler {
	return &EnergiaHandler{store: store}
}

// GetTarifas handles GET /v1/energia/tarifas.
//
// Optional query parameter:
//   - ?distribuidora=X  — filter by distributor name (case-insensitive substring match, DB-level)
//
// Without a filter, returns the 100 most-recent records as a sample.
// With ?distribuidora=, returns up to 1 000 matching records via DB-level JSONB filtering.
//
// Pricing: $0.001 USDC (+ $0.001 with ?format=context).
func (h *EnergiaHandler) GetTarifas(w http.ResponseWriter, r *http.Request) {
	filterDist := strings.TrimSpace(r.URL.Query().Get("distribuidora"))

	var records []domain.SourceRecord
	var err error
	if filterDist != "" {
		records, err = h.store.FindLatestFiltered(r.Context(), "aneel_tarifas", "distribuidora", filterDist)
	} else {
		records, err = h.store.FindLatest(r.Context(), "aneel_tarifas")
	}
	if err != nil {
		gatewayError(w, "energia", err)
		return
	}
	if len(records) == 0 {
		if filterDist != "" {
			jsonError(w, http.StatusNotFound, "no tariff records match the given filters")
		} else {
			jsonError(w, http.StatusNotFound, "ANEEL tariff data not yet available")
		}
		return
	}

	// Build a list representation suitable for the Data envelope.
	items := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		items = append(items, map[string]any{
			"record_key": rec.RecordKey,
			"fetched_at": rec.FetchedAt,
			"data":       rec.Data,
		})
	}

	updatedAt := records[0].FetchedAt

	respond(w, r, domain.APIResponse{
		Source:    "aneel_tarifas",
		UpdatedAt: updatedAt,
		CostUSDC:  "0.001",
		Data: map[string]any{
			"total":   len(records),
			"records": items,
		},
	})
}
