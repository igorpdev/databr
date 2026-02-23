package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// RiscoAmbientalHandler queries INPE deforestation data (DETER and PRODES)
// for a given municipality to produce an environmental risk assessment.
type RiscoAmbientalHandler struct {
	store      SourceStore
	httpClient *http.Client
}

// NewRiscoAmbientalHandler creates an environmental risk handler.
func NewRiscoAmbientalHandler(store SourceStore) *RiscoAmbientalHandler {
	return &RiscoAmbientalHandler{
		store:      store,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetHTTPClient overrides the HTTP client (for testing).
func (h *RiscoAmbientalHandler) SetHTTPClient(c *http.Client) { h.httpClient = c }

// GetRiscoAmbiental handles GET /v1/ambiental/risco/{municipio}.
// Accepts either an IBGE code (e.g., 1302603) or municipality name (e.g., Manaus).
func (h *RiscoAmbientalHandler) GetRiscoAmbiental(w http.ResponseWriter, r *http.Request) {
	municipio := chi.URLParam(r, "municipio")
	if municipio == "" {
		jsonError(w, http.StatusBadRequest, "municipality code or name is required")
		return
	}

	// DETER/PRODES data stores municipality names (lowercase), not IBGE codes
	municipioName := resolveIBGEToName(h.httpClient, municipio)

	ctx := r.Context()

	type queryResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		deterRes  queryResult
		prodesRes queryResult
		wg        sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		deterRes.records, deterRes.err = h.store.FindLatestFiltered(ctx, "inpe_deter", "municipio", municipioName)
	}()
	go func() {
		defer wg.Done()
		prodesRes.records, prodesRes.err = h.store.FindLatestFiltered(ctx, "inpe_prodes", "municipio", municipioName)
	}()
	wg.Wait()

	deterAlerts := []map[string]any{}
	if deterRes.err == nil {
		for _, rec := range deterRes.records {
			deterAlerts = append(deterAlerts, rec.Data)
		}
	}

	prodesData := []map[string]any{}
	if prodesRes.err == nil {
		for _, rec := range prodesRes.records {
			prodesData = append(prodesData, rec.Data)
		}
	}

	// Simple risk classification based on number of DETER alerts.
	riskLevel := "low"
	alertCount := len(deterAlerts)
	if alertCount >= 10 {
		riskLevel = "high"
	} else if alertCount >= 3 {
		riskLevel = "medium"
	}

	respond(w, r, domain.APIResponse{
		Source:   "risco_ambiental",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"municipio":     municipio,
			"risk_level":    riskLevel,
			"deter_alerts":  deterAlerts,
			"deter_count":   alertCount,
			"prodes_data":   prodesData,
			"prodes_count":  len(prodesData),
		},
	})
}
