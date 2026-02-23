package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/databr/api/internal/domain"
)

// PanoramaHandler aggregates multiple macroeconomic indicators into a single
// consolidated snapshot of the Brazilian economy.
type PanoramaHandler struct {
	store SourceStore
}

// NewPanoramaHandler creates a panorama handler.
func NewPanoramaHandler(store SourceStore) *PanoramaHandler {
	return &PanoramaHandler{store: store}
}

// GetPanorama handles GET /v1/economia/panorama.
func (h *PanoramaHandler) GetPanorama(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	sources := []string{
		"bcb_selic",
		"ibge_ipca",
		"ibge_pib",
		"bcb_ptax",
		"bcb_focus",
		"bcb_reservas",
		"bcb_credito",
	}

	type sourceResult struct {
		source  string
		records []domain.SourceRecord
		err     error
	}

	results := make([]sourceResult, len(sources))
	var wg sync.WaitGroup

	for i, src := range sources {
		wg.Add(1)
		go func(idx int, s string) {
			defer wg.Done()
			recs, err := h.store.FindLatest(ctx, s)
			results[idx] = sourceResult{source: s, records: recs, err: err}
		}(i, src)
	}
	wg.Wait()

	panorama := map[string]any{}
	var latestUpdate time.Time

	for _, res := range results {
		if res.err != nil || len(res.records) == 0 {
			panorama[res.source] = nil
			continue
		}
		rec := res.records[0]
		panorama[res.source] = rec.Data
		if rec.FetchedAt.After(latestUpdate) {
			latestUpdate = rec.FetchedAt
		}
	}

	respond(w, r, domain.APIResponse{
		Source:    "panorama_economico",
		UpdatedAt: latestUpdate,
		CostUSDC:  "0.010",
		Data:      panorama,
	})
}
