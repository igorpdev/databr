package handlers

import (
	"net/http"
	"sync"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// DueDiligenceHandler aggregates company, compliance, judicial, and government
// contract data to produce a risk assessment for a given CNPJ.
type DueDiligenceHandler struct {
	cnpjFetcher       CNPJFetcher
	complianceFetcher ComplianceFetcher
	judicialSearcher  DataJudSearcher
	store             SourceStore
}

// NewDueDiligenceHandler creates a due diligence handler.
func NewDueDiligenceHandler(
	cnpjFetcher CNPJFetcher,
	complianceFetcher ComplianceFetcher,
	judicialSearcher DataJudSearcher,
	store SourceStore,
) *DueDiligenceHandler {
	return &DueDiligenceHandler{
		cnpjFetcher:       cnpjFetcher,
		complianceFetcher: complianceFetcher,
		judicialSearcher:  judicialSearcher,
		store:             store,
	}
}

// GetDueDiligence handles GET /v1/empresas/{cnpj}/duediligence.
func (h *DueDiligenceHandler) GetDueDiligence(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if len(normalized) != 14 {
		jsonError(w, http.StatusBadRequest, "CNPJ must have 14 digits")
		return
	}

	ctx := r.Context()

	type companyResult struct {
		records []domain.SourceRecord
		err     error
	}
	type complianceResult struct {
		records []domain.SourceRecord
		err     error
	}
	type judicialResult struct {
		records []domain.SourceRecord
		err     error
	}
	type contractResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		companyRes    companyResult
		complianceRes complianceResult
		judicialRes   judicialResult
		contractRes   contractResult
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

	// Calculate risk score
	riskScore := 0

	// +20 if no company data found (suspicious)
	companyData := map[string]any{}
	if companyRes.err != nil || len(companyRes.records) == 0 {
		riskScore += 20
	} else {
		companyData = companyRes.records[0].Data
	}

	// +30 if compliance sanctions found
	sanctionsFound := false
	complianceData := []map[string]any{}
	if complianceRes.err == nil && len(complianceRes.records) > 0 {
		for _, rec := range complianceRes.records {
			complianceData = append(complianceData, rec.Data)
		}
		sanctionsFound = true
		riskScore += 30
	}

	// +5 per judicial process (up to 50)
	judicialData := []map[string]any{}
	if judicialRes.err == nil && len(judicialRes.records) > 0 {
		for _, rec := range judicialRes.records {
			judicialData = append(judicialData, rec.Data)
		}
		judicialPenalty := len(judicialRes.records) * 5
		if judicialPenalty > 50 {
			judicialPenalty = 50
		}
		riskScore += judicialPenalty
	}

	// Clamp to 0-100
	if riskScore > 100 {
		riskScore = 100
	}

	riskLevel := "low"
	if riskScore >= 60 {
		riskLevel = "high"
	} else if riskScore >= 30 {
		riskLevel = "medium"
	}

	// Government contracts
	contractData := []map[string]any{}
	if contractRes.err == nil {
		for _, rec := range contractRes.records {
			contractData = append(contractData, rec.Data)
		}
	}

	respond(w, r, domain.APIResponse{
		Source:   "duediligence",
		CostUSDC: "0.050",
		Data: map[string]any{
			"cnpj":                normalized,
			"risk_score":          riskScore,
			"risk_level":          riskLevel,
			"company":             companyData,
			"sanctions_found":     sanctionsFound,
			"compliance":          complianceData,
			"judicial_processes":  judicialData,
			"government_contracts": contractData,
		},
	})
}
