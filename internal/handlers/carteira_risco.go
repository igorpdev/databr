package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
)

const maxCarteiraCNPJs = 50

// carteiraRequest is the expected JSON body for POST /v1/carteira/risco.
type carteiraRequest struct {
	CNPJs []string `json:"cnpjs"`
}

// CarteiraRiscoHandler calculates portfolio risk for a list of CNPJs.
type CarteiraRiscoHandler struct {
	cnpjFetcher       CNPJFetcher
	complianceFetcher ComplianceFetcher
	store             SourceStore
}

// NewCarteiraRiscoHandler creates a carteira risco handler.
func NewCarteiraRiscoHandler(
	cnpjF CNPJFetcher,
	comp ComplianceFetcher,
	store SourceStore,
) *CarteiraRiscoHandler {
	return &CarteiraRiscoHandler{
		cnpjFetcher:       cnpjF,
		complianceFetcher: comp,
		store:             store,
	}
}

// cnpjRiskResult holds the individual risk assessment for a single CNPJ.
type cnpjRiskResult struct {
	CNPJ       string `json:"cnpj"`
	RiskScore  int    `json:"risk_score"`
	RiskLevel  string `json:"risk_level"`
	RazaoSocial string `json:"razao_social,omitempty"`
	CNAE       string `json:"cnae_principal,omitempty"`
	UF         string `json:"uf,omitempty"`
	CompanyAge int    `json:"company_age_years,omitempty"`
	Sanctioned bool   `json:"sanctioned"`
	Judicial   int    `json:"judicial_count"`
}

// PostCarteiraRisco handles POST /v1/carteira/risco.
func (h *CarteiraRiscoHandler) PostCarteiraRisco(w http.ResponseWriter, r *http.Request) {
	var req carteiraRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "corpo JSON inválido")
		return
	}

	if len(req.CNPJs) == 0 {
		jsonError(w, http.StatusBadRequest, "campo 'cnpjs' é obrigatório e não pode ser vazio")
		return
	}
	if len(req.CNPJs) > maxCarteiraCNPJs {
		jsonError(w, http.StatusBadRequest, "máximo de 50 CNPJs por requisição")
		return
	}

	// Normalize and validate all CNPJs upfront.
	normalized := make([]string, 0, len(req.CNPJs))
	for _, raw := range req.CNPJs {
		n := cnpj.NormalizeCNPJ(raw)
		if !isValidCNPJ(n) {
			jsonError(w, http.StatusBadRequest, "CNPJ inválido: "+raw)
			return
		}
		normalized = append(normalized, n)
	}

	ctx := r.Context()

	// Assess each CNPJ concurrently.
	results := make([]cnpjRiskResult, len(normalized))
	var wg sync.WaitGroup

	for i, doc := range normalized {
		wg.Add(1)
		go func(idx int, cnpjNum string) {
			defer wg.Done()

			result := cnpjRiskResult{
				CNPJ: cnpjNum,
			}
			score := 0

			// Fetch company data.
			companyRecords, err := h.cnpjFetcher.FetchByCNPJ(ctx, cnpjNum)
			if err != nil || len(companyRecords) == 0 {
				score += 20 // unable to retrieve company data
			} else {
				data := companyRecords[0].Data

				if rs, ok := data["razao_social"].(string); ok {
					result.RazaoSocial = rs
				}
				if c, ok := data["cnae_fiscal_principal"].(map[string]any); ok {
					if code, ok := c["codigo"].(string); ok {
						result.CNAE = code
					}
				}
				// Fallback: flat cnae field from minhareceita.org
				if result.CNAE == "" {
					if c, ok := data["cnae_fiscal"].(string); ok {
						result.CNAE = c
					}
				}
				if uf, ok := data["uf"].(string); ok {
					result.UF = uf
				}

				// Calculate company age from data_inicio_atividade.
				if dataInicio, ok := data["data_inicio_atividade"].(string); ok && dataInicio != "" {
					if t, err := time.Parse("2006-01-02", dataInicio); err == nil {
						years := int(time.Since(t).Hours() / 8760)
						result.CompanyAge = years
						// Young companies (< 2 years) are higher risk.
						if years < 2 {
							score += 10
						}
					}
				}

				// Inactive company.
				if sit, ok := data["situacao_cadastral"].(string); ok && sit != "ATIVA" && sit != "" {
					score += 10
				}
			}

			// Fetch compliance data.
			complianceRecords, err := h.complianceFetcher.FetchByCNPJ(ctx, cnpjNum)
			if err == nil && len(complianceRecords) > 0 {
				result.Sanctioned = true
				score += 35
			}

			// Fetch judicial data from store.
			judicialRecords, err := h.store.FindLatestFiltered(ctx, "datajud_cnj", "documento", cnpjNum)
			if err == nil && len(judicialRecords) > 0 {
				result.Judicial = len(judicialRecords)
				penalty := len(judicialRecords) * 5
				if penalty > 35 {
					penalty = 35
				}
				score += penalty
			}

			if score > 100 {
				score = 100
			}
			result.RiskScore = score
			result.RiskLevel = classifyRiskLevel(score)
			results[idx] = result
		}(i, doc)
	}

	wg.Wait()

	// Compute portfolio summary.
	totalScore := 0
	distribution := map[string]int{"baixo": 0, "medio": 0, "alto": 0, "critico": 0}
	sectorConcentration := map[string]int{}
	geoConcentration := map[string]int{}

	for _, res := range results {
		totalScore += res.RiskScore

		switch res.RiskLevel {
		case "baixo":
			distribution["baixo"]++
		case "medio":
			distribution["medio"]++
		case "alto":
			distribution["alto"]++
		case "critico":
			distribution["critico"]++
		}

		if res.CNAE != "" {
			sectorConcentration[res.CNAE]++
		}
		if res.UF != "" {
			geoConcentration[res.UF]++
		}
	}

	avgScore := 0
	if len(results) > 0 {
		avgScore = totalScore / len(results)
	}

	respond(w, r, domain.APIResponse{
		Source:    "carteira_risco",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"total_cnpjs":               len(results),
			"risco_medio":               avgScore,
			"nivel_risco_medio":         classifyRiskLevel(avgScore),
			"distribuicao_risco":        distribution,
			"concentracao_setorial":     sectorConcentration,
			"concentracao_geografica":   geoConcentration,
			"empresas":                  results,
		},
	})
}

// classifyRiskLevel converts a numeric risk score (0-100) to a textual level.
func classifyRiskLevel(score int) string {
	switch {
	case score >= 70:
		return "critico"
	case score >= 40:
		return "alto"
	case score >= 20:
		return "medio"
	default:
		return "baixo"
	}
}
