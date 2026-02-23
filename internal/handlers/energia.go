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
// Optional query parameters:
//   - ?uf=SP            — filter by state code (matched against the "uf" field in Data)
//   - ?distribuidora=X  — filter by distributor name (case-insensitive prefix match)
//
// Pricing: $0.001 USDC (+ $0.001 with ?format=context).
func (h *EnergiaHandler) GetTarifas(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "aneel_tarifas")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "ANEEL tariff data not yet available")
		return
	}

	// Apply optional filters from query parameters.
	filterUF := strings.TrimSpace(r.URL.Query().Get("uf"))
	filterDist := strings.TrimSpace(strings.ToUpper(r.URL.Query().Get("distribuidora")))

	filtered := make([]domain.SourceRecord, 0, len(records))
	for _, rec := range records {
		if filterUF != "" {
			uf, _ := rec.Data["uf"].(string)
			if !strings.EqualFold(uf, filterUF) {
				continue
			}
		}
		if filterDist != "" {
			dist, _ := rec.Data["distribuidora"].(string)
			if !strings.Contains(strings.ToUpper(dist), filterDist) {
				continue
			}
		}
		filtered = append(filtered, rec)
	}

	if len(filtered) == 0 {
		jsonError(w, http.StatusNotFound, "no tariff records match the given filters")
		return
	}

	// Build a list representation suitable for the Data envelope.
	items := make([]map[string]any, 0, len(filtered))
	for _, rec := range filtered {
		items = append(items, map[string]any{
			"record_key": rec.RecordKey,
			"fetched_at": rec.FetchedAt,
			"data":       rec.Data,
		})
	}

	updatedAt := filtered[0].FetchedAt

	respond(w, r, domain.APIResponse{
		Source:    "aneel_tarifas",
		UpdatedAt: updatedAt,
		CostUSDC:  "0.001",
		Data: map[string]any{
			"total":   len(filtered),
			"records": items,
		},
	})
}
