package handlers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// LitigioRiscoHandler provides a litigation risk assessment for a given CNPJ.
type LitigioRiscoHandler struct {
	judicialFetcher DataJudSearcher
	cnpjFetcher     CNPJFetcher
	store           SourceStore
}

// NewLitigioRiscoHandler creates a litigio risco handler.
func NewLitigioRiscoHandler(
	judicial DataJudSearcher,
	cnpjF CNPJFetcher,
	store SourceStore,
) *LitigioRiscoHandler {
	return &LitigioRiscoHandler{
		judicialFetcher: judicial,
		cnpjFetcher:     cnpjF,
		store:           store,
	}
}

// processTypeDistribution holds the count of processes by litigation type.
type processTypeDistribution struct {
	Trabalhista int `json:"trabalhista"`
	Civel       int `json:"civel"`
	Tributario  int `json:"tributario"`
	Criminal    int `json:"criminal"`
	Outros      int `json:"outros"`
}

// GetLitigioRisco handles GET /v1/litigio/{cnpj}/risco.
func (h *LitigioRiscoHandler) GetLitigioRisco(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if !isValidCNPJ(normalized) {
		jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos válidos")
		return
	}

	ctx := r.Context()

	type fetchResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		companyRes  fetchResult
		judicialRes fetchResult
		wg          sync.WaitGroup
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		companyRes.records, companyRes.err = h.cnpjFetcher.FetchByCNPJ(ctx, normalized)
	}()

	go func() {
		defer wg.Done()
		judicialRes.records, judicialRes.err = h.judicialFetcher.Search(ctx, normalized)
	}()

	wg.Wait()

	// Build empresa section.
	empresa := map[string]any{"cnpj": normalized}
	if companyRes.err == nil && len(companyRes.records) > 0 {
		data := companyRes.records[0].Data
		if rs, ok := data["razao_social"].(string); ok {
			empresa["razao_social"] = rs
		}
		if uf, ok := data["uf"].(string); ok {
			empresa["uf"] = uf
		}
		if cnae, ok := data["cnae_fiscal"].(string); ok {
			empresa["cnae_fiscal"] = cnae
		}
	}

	// If judicial fetch failed entirely, return what we have with zero risk.
	if judicialRes.err != nil {
		gatewayError(w, "litigio_risco", judicialRes.err)
		return
	}

	processes := judicialRes.records
	totalProcesses := len(processes)

	// Classify processes by type and role.
	dist := processTypeDistribution{}
	asDefendant := 0
	asPlaintiff := 0
	recentCount := 0 // last 12 months
	priorCount := 0  // older than 12 months
	cutoff := time.Now().UTC().AddDate(-1, 0, 0)

	processDetails := make([]map[string]any, 0, totalProcesses)

	for _, rec := range processes {
		processDetails = append(processDetails, rec.Data)

		// Classify by type using classe or assunto fields.
		classifyProcessType(&dist, rec.Data)

		// Determine role: defendant (réu/requerido) vs plaintiff (autor/requerente).
		role := extractRole(rec.Data, normalized)
		switch role {
		case "reu":
			asDefendant++
		case "autor":
			asPlaintiff++
		}

		// Classify by date for trend analysis.
		if dataDistribuicao, ok := rec.Data["dataAjuizamento"].(string); ok && dataDistribuicao != "" {
			if t, err := time.Parse("2006-01-02T15:04:05", dataDistribuicao); err == nil {
				if t.After(cutoff) {
					recentCount++
				} else {
					priorCount++
				}
			} else if t, err := time.Parse("2006-01-02", dataDistribuicao); err == nil {
				if t.After(cutoff) {
					recentCount++
				} else {
					priorCount++
				}
			}
		}
	}

	// Calculate litigation risk score (0-100).
	riskScore := calculateLitigationRisk(totalProcesses, asDefendant, recentCount, priorCount, &dist)

	// Determine trend.
	trend := "estavel"
	if recentCount > priorCount && priorCount > 0 {
		trend = "crescente"
	} else if recentCount < priorCount && recentCount >= 0 {
		trend = "decrescente"
	}

	respond(w, r, domain.APIResponse{
		Source:    "litigio_risco",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"empresa": empresa,
			"processos_resumo": map[string]any{
				"total":        totalProcesses,
				"como_reu":     asDefendant,
				"como_autor":   asPlaintiff,
				"recentes_12m": recentCount,
				"anteriores":   priorCount,
			},
			"risco_litigio": map[string]any{
				"score":       riskScore,
				"nivel":       classifyRiskLevel(riskScore),
			},
			"distribuicao_tipos": map[string]any{
				"trabalhista": dist.Trabalhista,
				"civel":       dist.Civel,
				"tributario":  dist.Tributario,
				"criminal":    dist.Criminal,
				"outros":      dist.Outros,
			},
			"tendencia": map[string]any{
				"direcao":      trend,
				"recentes_12m": recentCount,
				"anteriores":   priorCount,
			},
			"processos": processDetails,
		},
	})
}

// classifyProcessType categorizes a judicial process into one of the main types
// based on the classe_processual, assunto, or orgao_julgador fields.
func classifyProcessType(dist *processTypeDistribution, data map[string]any) {
	// Try to classify from classe or assunto text.
	text := strings.ToLower(extractString(data, "classeProcessual") +
		" " + extractString(data, "assunto") +
		" " + extractString(data, "orgaoJulgador"))

	switch {
	case strings.Contains(text, "trabalh") || strings.Contains(text, "trt") || strings.Contains(text, "tst"):
		dist.Trabalhista++
	case strings.Contains(text, "tribut") || strings.Contains(text, "fiscal") || strings.Contains(text, "execução fiscal"):
		dist.Tributario++
	case strings.Contains(text, "criminal") || strings.Contains(text, "penal") || strings.Contains(text, "crime"):
		dist.Criminal++
	case strings.Contains(text, "cível") || strings.Contains(text, "civel") || strings.Contains(text, "indeniz"):
		dist.Civel++
	default:
		dist.Outros++
	}
}

// extractRole determines whether the given CNPJ appears as defendant or plaintiff.
func extractRole(data map[string]any, cnpjNum string) string {
	// DataJud records may have "polo_ativo" and "polo_passivo" arrays.
	if isInPolo(data, "poloPassivo", cnpjNum) {
		return "reu"
	}
	if isInPolo(data, "poloAtivo", cnpjNum) {
		return "autor"
	}
	return ""
}

// isInPolo checks if a given CNPJ appears in the specified polo (party list).
func isInPolo(data map[string]any, poloKey, cnpjNum string) bool {
	polo, ok := data[poloKey]
	if !ok {
		return false
	}
	poloSlice, ok := polo.([]any)
	if !ok {
		return false
	}
	for _, entry := range poloSlice {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if doc, ok := m["documento"].(string); ok {
			cleaned := reDigits.ReplaceAllString(doc, "")
			if cleaned == cnpjNum {
				return true
			}
		}
		if doc, ok := m["cpf_cnpj"].(string); ok {
			cleaned := reDigits.ReplaceAllString(doc, "")
			if cleaned == cnpjNum {
				return true
			}
		}
	}
	return false
}

// calculateLitigationRisk computes a 0-100 risk score based on litigation metrics.
func calculateLitigationRisk(total, asDefendant, recent, prior int, dist *processTypeDistribution) int {
	score := 0

	// Base score from total processes.
	switch {
	case total >= 50:
		score += 30
	case total >= 20:
		score += 20
	case total >= 5:
		score += 10
	case total >= 1:
		score += 5
	}

	// Defendant ratio penalty: more defendant appearances = higher risk.
	if total > 0 {
		defendantRatio := float64(asDefendant) / float64(total)
		if defendantRatio >= 0.8 {
			score += 20
		} else if defendantRatio >= 0.5 {
			score += 10
		}
	}

	// Trend penalty: growing litigation is higher risk.
	if prior > 0 && recent > prior {
		growth := float64(recent-prior) / float64(prior)
		if growth >= 1.0 {
			score += 20 // doubled or more
		} else if growth >= 0.5 {
			score += 10
		}
	} else if prior == 0 && recent > 0 {
		score += 15 // new litigation appearing
	}

	// Criminal cases are high risk.
	if dist.Criminal > 0 {
		score += 15
	}

	// Tax (tributario) cases add moderate risk.
	if dist.Tributario >= 3 {
		score += 10
	} else if dist.Tributario >= 1 {
		score += 5
	}

	if score > 100 {
		score = 100
	}
	return score
}

// extractString safely extracts a string value from a map.
func extractString(data map[string]any, key string) string {
	v, ok := data[key].(string)
	if !ok {
		return ""
	}
	return v
}
