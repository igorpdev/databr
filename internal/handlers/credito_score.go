package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// CreditoScoreHandler aggregates multiple data sources to produce a credit
// score estimate for a given CNPJ.
type CreditoScoreHandler struct {
	cnpjFetcher       CNPJFetcher
	complianceFetcher ComplianceFetcher
	judicialSearcher  DataJudSearcher
	store             SourceStore
}

// NewCreditoScoreHandler creates a credit score handler.
func NewCreditoScoreHandler(
	cnpjFetcher CNPJFetcher,
	complianceFetcher ComplianceFetcher,
	judicialSearcher DataJudSearcher,
	store SourceStore,
) *CreditoScoreHandler {
	return &CreditoScoreHandler{
		cnpjFetcher:       cnpjFetcher,
		complianceFetcher: complianceFetcher,
		judicialSearcher:  judicialSearcher,
		store:             store,
	}
}

// GetCreditoScore handles GET /v1/credito/score/{cnpj}.
func (h *CreditoScoreHandler) GetCreditoScore(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if len(normalized) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ must have 14 digits")
		return
	}

	ctx := r.Context()

	type queryResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		companyRes    queryResult
		complianceRes queryResult
		judicialRes   queryResult
		contractRes   queryResult
		wg            sync.WaitGroup
	)

	wg.Add(4)
	go func() {
		defer wg.Done()
		companyRes.records, companyRes.err = h.cnpjFetcher.FetchByCNPJ(ctx, normalized)
	}()
	go func() {
		defer wg.Done()
		complianceRes.records, complianceRes.err = h.complianceFetcher.FetchByCNPJ(ctx, normalized)
	}()
	go func() {
		defer wg.Done()
		judicialRes.records, judicialRes.err = h.judicialSearcher.Search(ctx, normalized)
	}()
	go func() {
		defer wg.Done()
		// PNCP stores orgao (buyer), not supplier CNPJ — search by cnpj_orgao
		contractRes.records, contractRes.err = h.store.FindLatestFiltered(ctx, "pncp_licitacoes", "cnpj_orgao", normalized)
	}()
	wg.Wait()

	// Start at 70
	score := 70
	factors := []string{}

	// -30 if sanctions found
	sanctionsFound := false
	if complianceRes.err == nil && len(complianceRes.records) > 0 {
		sanctionsFound = true
		score -= 30
		factors = append(factors, "sanctions_penalty_-30")
	}

	// -5 per judicial process
	judicialCount := 0
	if judicialRes.err == nil {
		judicialCount = len(judicialRes.records)
		penalty := judicialCount * 5
		score -= penalty
		if judicialCount > 0 {
			factors = append(factors, "judicial_penalty")
		}
	}

	// +10 if active government contracts
	hasContracts := false
	if contractRes.err == nil && len(contractRes.records) > 0 {
		hasContracts = true
		score += 10
		factors = append(factors, "government_contracts_+10")
	}

	// +10 if company age > 5 years
	companyAge := 0
	if companyRes.err == nil && len(companyRes.records) > 0 {
		if dateStr, ok := companyRes.records[0].Data["data_inicio_atividade"].(string); ok && dateStr != "" {
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				companyAge = int(time.Since(t).Hours() / (24 * 365.25))
				if companyAge > 5 {
					score += 10
					factors = append(factors, "company_age_bonus_+10")
				}
			}
		}
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	// Determine rating
	rating := "C"
	if score >= 80 {
		rating = "A"
	} else if score >= 60 {
		rating = "B"
	}

	respond(w, r, domain.APIResponse{
		Source:   "credito_score",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"cnpj":               normalized,
			"score":              score,
			"rating":             rating,
			"factors":            factors,
			"sanctions_found":    sanctionsFound,
			"judicial_count":     judicialCount,
			"has_gov_contracts":  hasContracts,
			"company_age_years":  companyAge,
		},
	})
}
