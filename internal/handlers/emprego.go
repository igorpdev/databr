package handlers

import (
	"net/http"
	"strings"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
)

// EmpregoHandler handles requests for /v1/emprego/*.
type EmpregoHandler struct {
	store SourceStore
}

// NewEmpregoHandler creates an EmpregoHandler backed by the given SourceStore.
func NewEmpregoHandler(store SourceStore) *EmpregoHandler {
	return &EmpregoHandler{store: store}
}

// GetCAGED handles GET /v1/emprego/caged.
// Supports ?uf=XX to filter by state and ?mes=YYYYMM to select a specific period.
func (h *EmpregoHandler) GetCAGED(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ufFilter := strings.ToUpper(strings.TrimSpace(q.Get("uf")))

	records, err := h.store.FindLatest(r.Context(), "caged_emprego")
	if err != nil {
		gatewayError(w, "caged_emprego", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "caged data not yet available")
		return
	}

	// Select specific period or use latest (first record).
	rec := records[0]
	if mes := q.Get("mes"); mes != "" {
		found := false
		for _, r := range records {
			if r.RecordKey == mes {
				rec = r
				found = true
				break
			}
		}
		if !found {
			jsonError(w, http.StatusNotFound, "caged data for period "+mes+" not found")
			return
		}
	}

	data := filterEmpregoItems(rec.Data, ufFilter)

	respond(w, r, domain.APIResponse{
		Source:    "caged_emprego",
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      data,
	})
}

// GetRAIS handles GET /v1/emprego/rais.
// Supports ?uf=XX to filter by state and ?ano=YYYY to select a specific year.
func (h *EmpregoHandler) GetRAIS(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ufFilter := strings.ToUpper(strings.TrimSpace(q.Get("uf")))

	records, err := h.store.FindLatest(r.Context(), "rais_emprego")
	if err != nil {
		gatewayError(w, "rais_emprego", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "rais data not yet available")
		return
	}

	// Select specific year or use latest (first record).
	rec := records[0]
	if ano := q.Get("ano"); ano != "" {
		found := false
		for _, r := range records {
			if r.RecordKey == ano {
				rec = r
				found = true
				break
			}
		}
		if !found {
			jsonError(w, http.StatusNotFound, "rais data for year "+ano+" not found")
			return
		}
	}

	data := filterEmpregoItems(rec.Data, ufFilter)

	respond(w, r, domain.APIResponse{
		Source:    "rais_emprego",
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      data,
	})
}

// filterEmpregoItems extracts items from data and optionally filters by UF.
func filterEmpregoItems(data map[string]any, ufFilter string) map[string]any {
	items := extractItems(data)
	if items == nil {
		return data
	}

	if ufFilter != "" {
		filtered := make([]map[string]any, 0)
		for _, item := range items {
			if uf, _ := item["uf"].(string); strings.EqualFold(uf, ufFilter) {
				filtered = append(filtered, item)
			}
		}
		// Preserve other fields from data, replace items and total.
		result := make(map[string]any)
		for k, v := range data {
			result[k] = v
		}
		result["items"] = filtered
		result["total"] = len(filtered)
		return result
	}

	return data
}

// extractItems handles both in-memory ([]map[string]any) and JSONB-deserialized ([]any) types.
func extractItems(data map[string]any) []map[string]any {
	switch v := data["items"].(type) {
	case []map[string]any:
		return v
	case []any:
		result := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			}
		}
		return result
	}
	return nil
}
