package handlers

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// SetorHandler fetches a company and then enriches it with sector-specific
// data based on the CNAE code.
type SetorHandler struct {
	cnpjFetcher CNPJFetcher
	store       SourceStore
}

// NewSetorHandler creates a sector analysis handler.
func NewSetorHandler(cnpjFetcher CNPJFetcher, store SourceStore) *SetorHandler {
	return &SetorHandler{cnpjFetcher: cnpjFetcher, store: store}
}

// GetSetor handles GET /v1/empresas/{cnpj}/setor.
func (h *SetorHandler) GetSetor(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if len(normalized) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ must have 14 digits")
		return
	}

	ctx := r.Context()

	records, err := h.cnpjFetcher.FetchByCNPJ(ctx, normalized)
	if err != nil {
		gatewayError(w, "setor", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "CNPJ not found")
		return
	}

	rec := records[0]

	// Extract CNAE code from company data. It may be a string or a number.
	cnaeCode := extractStringOrNumber(rec.Data, "cnae_fiscal")
	if cnaeCode == "" {
		cnaeCode = extractStringOrNumber(rec.Data, "cnae_fiscal_principal")
	}

	// Detect publicly traded via natureza jurídica:
	// 2046 = Sociedade Anônima Aberta (S.A. de capital aberto)
	// 2038 = Sociedade de Economia Mista (state-owned, listed e.g. Petrobras, BB)
	natJuridica := extractStringOrNumber(rec.Data, "codigo_natureza_juridica")
	if natJuridica == "" {
		natJuridica = extractStringOrNumber(rec.Data, "natureza_juridica")
	}
	publiclyTraded := natJuridica == "2046" || natJuridica == "2038"

	// Parallel sector data enrichment.
	type queryResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		ibgeRes queryResult
		b3Res   queryResult
		wg      sync.WaitGroup
	)

	if cnaeCode != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ibgeRes.records, ibgeRes.err = h.store.FindLatestFiltered(ctx, "ibge_pib", "cnae", cnaeCode)
		}()
	}

	// Search B3 data by company name (B3 stores tickers, not CNPJs)
	razaoSocial, _ := rec.Data["razao_social"].(string)
	if razaoSocial != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b3Res.records, b3Res.err = h.store.FindLatestFiltered(ctx, "b3_cotacoes", "ticker", "")
		}()
	}

	wg.Wait()

	sectorData := map[string]any{}
	if ibgeRes.err == nil && len(ibgeRes.records) > 0 {
		items := make([]map[string]any, len(ibgeRes.records))
		for i, r := range ibgeRes.records {
			items[i] = r.Data
		}
		sectorData["ibge_sector_data"] = items
	}

	if b3Res.err == nil && len(b3Res.records) > 0 {
		items := make([]map[string]any, len(b3Res.records))
		for i, r := range b3Res.records {
			items[i] = r.Data
		}
		sectorData["b3_cotacoes"] = items
	}

	respond(w, r, domain.APIResponse{
		Source:    "setor_analise",
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.030",
		Data: map[string]any{
			"cnpj":            normalized,
			"razao_social":    rec.Data["razao_social"],
			"cnae_fiscal":     cnaeCode,
			"publicly_traded": publiclyTraded,
			"sector_data":     sectorData,
		},
	})
}

// extractStringOrNumber tries to get a string value from a map; if the value
// is a float64 (common after JSON unmarshalling), it converts to a string
// without decimals.
func extractStringOrNumber(data map[string]any, key string) string {
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%.0f", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
