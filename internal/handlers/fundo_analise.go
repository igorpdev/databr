package handlers

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// FundoAnaliseHandler aggregates CVM fund data with BCB/IBGE indicators
// to compute performance metrics for a given investment fund.
type FundoAnaliseHandler struct {
	store SourceStore
}

// NewFundoAnaliseHandler creates a fund analysis handler.
func NewFundoAnaliseHandler(store SourceStore) *FundoAnaliseHandler {
	return &FundoAnaliseHandler{store: store}
}

// GetFundoAnalise handles GET /v1/mercado/fundos/{cnpj}/analise.
func (h *FundoAnaliseHandler) GetFundoAnalise(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if len(normalized) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ must have 14 digits")
		return
	}

	ctx := r.Context()

	type queryResult struct {
		record  *domain.SourceRecord
		records []domain.SourceRecord
		err     error
	}

	var (
		fundoRes queryResult
		cotasRes queryResult
		selicRes queryResult
		ipcaRes  queryResult
		wg       sync.WaitGroup
	)

	wg.Add(4)
	go func() {
		defer wg.Done()
		fundoRes.record, fundoRes.err = h.store.FindOne(ctx, "cvm_fundos", normalized)
	}()
	go func() {
		defer wg.Done()
		cotasRes.records, cotasRes.err = h.store.FindLatestFiltered(ctx, "cvm_cotas", "cnpj", normalized)
	}()
	go func() {
		defer wg.Done()
		selicRes.records, selicRes.err = h.store.FindLatest(ctx, "bcb_selic")
	}()
	go func() {
		defer wg.Done()
		ipcaRes.records, ipcaRes.err = h.store.FindLatest(ctx, "ibge_ipca")
	}()
	wg.Wait()

	if fundoRes.err != nil {
		jsonError(w, http.StatusBadGateway, "failed to fetch fund data: "+fundoRes.err.Error())
		return
	}
	if fundoRes.record == nil {
		jsonError(w, http.StatusNotFound, "Fund not found for CNPJ "+normalized)
		return
	}

	fundoData := fundoRes.record.Data

	// Extract cotas data for performance calculation.
	cotasData := []map[string]any{}
	if cotasRes.err == nil {
		for _, rec := range cotasRes.records {
			cotasData = append(cotasData, rec.Data)
		}
	}

	// Extract selic rate for CDI comparison.
	var selicRate float64
	if selicRes.err == nil && len(selicRes.records) > 0 {
		selicRate = toFloat64(selicRes.records[0].Data["valor"])
	}

	// Extract IPCA for real return calculation.
	var ipcaRate float64
	if ipcaRes.err == nil && len(ipcaRes.records) > 0 {
		ipcaRate = toFloat64(ipcaRes.records[0].Data["valor"])
	}

	// Calculate performance metrics from cotas if available.
	var nominalReturn, realReturn, vsCDI float64
	if len(cotasData) >= 2 {
		// Use first and last cota values for return calculation.
		first := toFloat64(cotasData[len(cotasData)-1]["valor_cota"])
		last := toFloat64(cotasData[0]["valor_cota"])
		if first > 0 {
			nominalReturn = ((last - first) / first) * 100
			realReturn = nominalReturn - ipcaRate
			vsCDI = nominalReturn - selicRate
		}
	}

	respond(w, r, domain.APIResponse{
		Source:    "fundo_analise",
		UpdatedAt: fundoRes.record.FetchedAt,
		CostUSDC:  "0.050",
		Data: map[string]any{
			"cnpj":           normalized,
			"fundo":          fundoData,
			"cotas":          cotasData,
			"cotas_count":    len(cotasData),
			"selic_rate":     selicRate,
			"ipca_rate":      ipcaRate,
			"nominal_return": nominalReturn,
			"real_return":    realReturn,
			"vs_cdi":         vsCDI,
		},
	})
}

// toFloat64 tries to convert an interface value to float64.
func toFloat64(v any) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}
