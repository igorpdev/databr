package handlers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// ESGHandler builds an ESG (Environmental, Social, Governance) report for a
// given company CNPJ by aggregating data from multiple sources.
type ESGHandler struct {
	cnpjFetcher       CNPJFetcher
	complianceFetcher ComplianceFetcher
	store             SourceStore
}

// NewESGHandler creates an ESG report handler.
func NewESGHandler(cnpjFetcher CNPJFetcher, complianceFetcher ComplianceFetcher, store SourceStore) *ESGHandler {
	return &ESGHandler{
		cnpjFetcher:       cnpjFetcher,
		complianceFetcher: complianceFetcher,
		store:             store,
	}
}

// GetESG handles GET /v1/ambiental/empresa/{cnpj}/esg.
func (h *ESGHandler) GetESG(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if !isValidCNPJ(normalized) {
		jsonError(w, http.StatusBadRequest, "CNPJ inválido — informe 14 dígitos válidos")
		return
	}

	ctx := r.Context()

	// Step 1: Fetch company data for location and sector info.
	companyRecords, err := h.cnpjFetcher.FetchByCNPJ(ctx, normalized)
	if err != nil {
		gatewayError(w, "esg", err)
		return
	}
	if len(companyRecords) == 0 {
		jsonError(w, http.StatusNotFound, "CNPJ não encontrado")
		return
	}

	companyData := companyRecords[0].Data

	// Extract location info for environmental queries.
	municipio, _ := companyData["municipio"].(string)
	uf, _ := companyData["uf"].(string)
	razaoSocial, _ := companyData["razao_social"].(string)

	// Step 2: Parallel queries for E, S, G data.
	type queryResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		ibamaRes     queryResult
		deterRes     queryResult
		complianceRes queryResult
		pncpRes      queryResult
		wg           sync.WaitGroup
	)

	// Environmental queries.
	if h.store != nil {
		wg.Add(2)
		go func() {
			defer wg.Done()
			// Search IBAMA embargos by CNPJ or company name.
			ibamaRes.records, ibamaRes.err = h.store.FindLatestFiltered(ctx, "ibama_embargos", "cpf_cnpj", normalized)
		}()
		go func() {
			defer wg.Done()
			// Search DETER deforestation alerts for company's municipality.
			if municipio != "" {
				deterRes.records, deterRes.err = h.store.FindLatestFiltered(ctx, "inpe_deter", "municipio", municipio)
			}
		}()
	}

	// Social: compliance check.
	wg.Add(1)
	go func() {
		defer wg.Done()
		complianceRes.records, complianceRes.err = h.complianceFetcher.FetchByCNPJ(ctx, normalized)
	}()

	// Governance: government contracts.
	if h.store != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pncpRes.records, pncpRes.err = h.store.FindLatestFiltered(ctx, "pncp_licitacoes", "cnpj_orgao", normalized)
		}()
	}

	wg.Wait()

	// ---------- Environmental Score (E) ----------
	envScore := 100
	envDetalhes := map[string]any{}

	// IBAMA embargos.
	ibamaEmbargos := []map[string]any{}
	if ibamaRes.err == nil && len(ibamaRes.records) > 0 {
		for _, rec := range ibamaRes.records {
			ibamaEmbargos = append(ibamaEmbargos, rec.Data)
		}
		// Deduct 25 points per embargo (max deduction 60).
		penalty := len(ibamaEmbargos) * 25
		if penalty > 60 {
			penalty = 60
		}
		envScore -= penalty
	}
	envDetalhes["ibama_embargos"] = len(ibamaEmbargos)

	// DETER deforestation alerts near the company's municipality.
	deterAlerts := 0
	if deterRes.err == nil {
		deterAlerts = len(deterRes.records)
		// Deduct 5 points per alert (max deduction 30).
		penalty := deterAlerts * 5
		if penalty > 30 {
			penalty = 30
		}
		envScore -= penalty
	}
	envDetalhes["deter_alertas_municipio"] = deterAlerts
	envDetalhes["municipio"] = municipio

	if envScore < 0 {
		envScore = 0
	}

	// ---------- Social Score (S) ----------
	socialScore := 100
	socialDetalhes := map[string]any{}

	// Compliance sanctions (CEIS/CNEP).
	sancoes := []map[string]any{}
	if complianceRes.err == nil && len(complianceRes.records) > 0 {
		for _, rec := range complianceRes.records {
			sancoes = append(sancoes, rec.Data)
		}
		// Each sanction deducts 20 points (max deduction 80).
		penalty := len(sancoes) * 20
		if penalty > 80 {
			penalty = 80
		}
		socialScore -= penalty
	}
	socialDetalhes["sancoes_compliance"] = len(sancoes)
	socialDetalhes["listas_verificadas"] = []string{"CEIS", "CNEP", "CEPIM"}

	if socialScore < 0 {
		socialScore = 0
	}

	// ---------- Governance Score (G) ----------
	govScore := 100
	govDetalhes := map[string]any{}

	// QSA (board/partner composition) from CNPJ data.
	qsaCount := 0
	if qsa, ok := companyData["qsa"].([]any); ok {
		qsaCount = len(qsa)
	}
	govDetalhes["socios_qsa"] = qsaCount

	// Assess governance based on partner composition.
	if qsaCount == 0 {
		// No QSA data — minor governance concern.
		govScore -= 10
	} else if qsaCount == 1 {
		// Single partner — higher concentration risk.
		govScore -= 5
	}

	// Government contracts (transparency).
	contratos := []map[string]any{}
	if pncpRes.err == nil && len(pncpRes.records) > 0 {
		for _, rec := range pncpRes.records {
			contratos = append(contratos, rec.Data)
		}
	}
	govDetalhes["contratos_governo"] = len(contratos)

	// Check company status — inactive companies get governance penalty.
	situacao, _ := companyData["situacao_cadastral"].(string)
	if situacao != "" && !strings.EqualFold(situacao, "ATIVA") && !strings.EqualFold(situacao, "ativa") {
		govScore -= 20
		govDetalhes["situacao_cadastral"] = situacao
	}

	if govScore < 0 {
		govScore = 0
	}

	// ---------- Composite ESG Score ----------
	// Weighted: E=40%, S=30%, G=30%.
	esgScore := float64(envScore)*0.4 + float64(socialScore)*0.3 + float64(govScore)*0.3
	esgScoreInt := int(esgScore)

	// Classification: A (80-100), B (60-79), C (40-59), D (0-39).
	classificacao := "D"
	switch {
	case esgScoreInt >= 80:
		classificacao = "A"
	case esgScoreInt >= 60:
		classificacao = "B"
	case esgScoreInt >= 40:
		classificacao = "C"
	}

	respond(w, r, domain.APIResponse{
		Source:    "esg_report",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.020",
		Data: map[string]any{
			"empresa": map[string]any{
				"cnpj":         normalized,
				"razao_social": razaoSocial,
				"municipio":    municipio,
				"uf":           uf,
			},
			"esg_score":     esgScoreInt,
			"classificacao": classificacao,
			"ambiental": map[string]any{
				"score":    envScore,
				"peso":     "40%",
				"detalhes": envDetalhes,
			},
			"social": map[string]any{
				"score":    socialScore,
				"peso":     "30%",
				"detalhes": socialDetalhes,
			},
			"governanca": map[string]any{
				"score":    govScore,
				"peso":     "30%",
				"detalhes": govDetalhes,
			},
		},
	})
}
