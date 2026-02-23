package handlers

import (
	"net/http"
	"sync"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// ComplianceEleitoralHandler aggregates compliance, judicial, and electoral
// candidate data to produce an electoral compliance assessment.
type ComplianceEleitoralHandler struct {
	complianceFetcher ComplianceFetcher
	judicialSearcher  DataJudSearcher
	store             SourceStore
}

// NewComplianceEleitoralHandler creates an electoral compliance handler.
func NewComplianceEleitoralHandler(
	complianceFetcher ComplianceFetcher,
	judicialSearcher DataJudSearcher,
	store SourceStore,
) *ComplianceEleitoralHandler {
	return &ComplianceEleitoralHandler{
		complianceFetcher: complianceFetcher,
		judicialSearcher:  judicialSearcher,
		store:             store,
	}
}

// GetComplianceEleitoral handles GET /v1/eleicoes/compliance/{cpf_cnpj}.
func (h *ComplianceEleitoralHandler) GetComplianceEleitoral(w http.ResponseWriter, r *http.Request) {
	doc := chi.URLParam(r, "cpf_cnpj")
	digits := reDigits.ReplaceAllString(doc, "")

	if len(digits) != 11 && len(digits) != 14 {
		jsonError(w, http.StatusBadRequest, "CPF (11 digits) or CNPJ (14 digits) is required")
		return
	}

	ctx := r.Context()

	type queryResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		complianceRes queryResult
		judicialRes   queryResult
		tseRes        queryResult
		wg            sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		complianceRes.records, complianceRes.err = h.complianceFetcher.FetchByCNPJ(ctx, digits)
	}()
	go func() {
		defer wg.Done()
		judicialRes.records, judicialRes.err = h.judicialSearcher.Search(ctx, digits)
	}()
	go func() {
		defer wg.Done()
		tseRes.records, tseRes.err = h.store.FindLatestFiltered(ctx, "tse_candidatos", "cpf_cnpj", digits)
	}()
	wg.Wait()

	complianceData := []map[string]any{}
	sanctionsFound := false
	if complianceRes.err == nil && len(complianceRes.records) > 0 {
		sanctionsFound = true
		for _, rec := range complianceRes.records {
			complianceData = append(complianceData, rec.Data)
		}
	}

	judicialData := []map[string]any{}
	if judicialRes.err == nil {
		for _, rec := range judicialRes.records {
			judicialData = append(judicialData, rec.Data)
		}
	}

	candidateData := []map[string]any{}
	if tseRes.err == nil {
		for _, rec := range tseRes.records {
			candidateData = append(candidateData, rec.Data)
		}
	}

	// Determine electoral fitness status.
	status := "apto"
	if sanctionsFound || len(judicialData) > 0 {
		status = "requer_analise"
	}

	respond(w, r, domain.APIResponse{
		Source:   "compliance_eleitoral",
		CostUSDC: "0.030",
		Data: map[string]any{
			"documento":          digits,
			"status":             status,
			"sanctions_found":    sanctionsFound,
			"compliance":         complianceData,
			"judicial_processes": judicialData,
			"judicial_count":     len(judicialData),
			"candidate_records":  candidateData,
			"candidate_count":    len(candidateData),
		},
	})
}
