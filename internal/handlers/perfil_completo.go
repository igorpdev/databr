package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// DataJudSearcher is defined in judicial.go

// PerfilCompletoHandler aggregates cadastral, compliance, judicial, government
// contract, and environmental data into a single comprehensive company profile.
type PerfilCompletoHandler struct {
	cnpjFetcher       CNPJFetcher
	complianceFetcher ComplianceFetcher
	judicialFetcher   DataJudSearcher
	store             SourceStore
}

// NewPerfilCompletoHandler creates a perfil completo handler.
func NewPerfilCompletoHandler(
	cnpj CNPJFetcher,
	comp ComplianceFetcher,
	judicial DataJudSearcher,
	store SourceStore,
) *PerfilCompletoHandler {
	return &PerfilCompletoHandler{
		cnpjFetcher:       cnpj,
		complianceFetcher: comp,
		judicialFetcher:   judicial,
		store:             store,
	}
}

// GetPerfilCompleto handles GET /v1/empresas/{cnpj}/perfil-completo.
func (h *PerfilCompletoHandler) GetPerfilCompleto(w http.ResponseWriter, r *http.Request) {
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
		cadastroRes   fetchResult
		complianceRes fetchResult
		judicialRes   fetchResult
		contratosRes  fetchResult
		ambientalRes  fetchResult
		wg            sync.WaitGroup
	)

	wg.Add(5)

	go func() {
		defer wg.Done()
		cadastroRes.records, cadastroRes.err = h.cnpjFetcher.FetchByCNPJ(ctx, normalized)
	}()

	go func() {
		defer wg.Done()
		complianceRes.records, complianceRes.err = h.complianceFetcher.FetchByCNPJ(ctx, normalized)
	}()

	go func() {
		defer wg.Done()
		judicialRes.records, judicialRes.err = h.judicialFetcher.Search(ctx, normalized)
	}()

	go func() {
		defer wg.Done()
		contratosRes.records, contratosRes.err = h.store.FindLatestFiltered(ctx, "pncp_licitacoes", "cnpj_orgao", normalized)
	}()

	go func() {
		defer wg.Done()
		ambientalRes.records, ambientalRes.err = h.store.FindLatestFiltered(ctx, "ibama_embargos", "cpf_cnpj", normalized)
	}()

	wg.Wait()

	// --- Build cadastro section ---
	cadastroData := map[string]any{}
	cadastroStatus := "limpo"
	if cadastroRes.err != nil || len(cadastroRes.records) == 0 {
		cadastroStatus = "alerta" // unable to retrieve registration data
	} else {
		cadastroData = cadastroRes.records[0].Data
		// Check situacao_cadastral for inactive/suspended companies
		if sit, ok := cadastroData["situacao_cadastral"].(string); ok && sit != "ATIVA" && sit != "" {
			cadastroStatus = "alerta"
		}
	}

	// --- Build compliance section ---
	complianceStatus := "limpo"
	complianceSanctions := make([]map[string]any, 0)
	if complianceRes.err == nil && len(complianceRes.records) > 0 {
		for _, rec := range complianceRes.records {
			complianceSanctions = append(complianceSanctions, rec.Data)
		}
		complianceStatus = "critico"
	}

	// --- Build judicial section ---
	judicialStatus := "limpo"
	judicialProcesses := make([]map[string]any, 0)
	if judicialRes.err == nil && len(judicialRes.records) > 0 {
		for _, rec := range judicialRes.records {
			judicialProcesses = append(judicialProcesses, rec.Data)
		}
		count := len(judicialProcesses)
		if count >= 10 {
			judicialStatus = "critico"
		} else if count >= 1 {
			judicialStatus = "alerta"
		}
	}

	// --- Build contratos_governo section ---
	contratosStatus := "limpo"
	contratos := make([]map[string]any, 0)
	if contratosRes.err == nil && len(contratosRes.records) > 0 {
		for _, rec := range contratosRes.records {
			contratos = append(contratos, rec.Data)
		}
		// Having government contracts is informational, not necessarily risky
		contratosStatus = "limpo"
	}

	// --- Build ambiental section ---
	ambientalStatus := "limpo"
	embargos := make([]map[string]any, 0)
	if ambientalRes.err == nil && len(ambientalRes.records) > 0 {
		for _, rec := range ambientalRes.records {
			embargos = append(embargos, rec.Data)
		}
		ambientalStatus = "critico"
	}

	// --- Calculate overall risk score (0-100) ---
	riskScore := 0

	// +15 if unable to retrieve cadastral data
	if cadastroRes.err != nil || len(cadastroRes.records) == 0 {
		riskScore += 15
	}

	// +5 if company is not active
	if cadastroStatus == "alerta" {
		riskScore += 5
	}

	// +30 if compliance sanctions found
	if complianceStatus == "critico" {
		riskScore += 30
	}

	// +3 per judicial process (up to 30)
	judicialPenalty := len(judicialProcesses) * 3
	if judicialPenalty > 30 {
		judicialPenalty = 30
	}
	riskScore += judicialPenalty

	// +25 if IBAMA embargos found
	if ambientalStatus == "critico" {
		riskScore += 25
	}

	if riskScore > 100 {
		riskScore = 100
	}

	respond(w, r, domain.APIResponse{
		Source:    "perfil_completo",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.015",
		Data: map[string]any{
			"cnpj":       normalized,
			"risk_score": riskScore,
			"cadastro": map[string]any{
				"status": cadastroStatus,
				"dados":  cadastroData,
			},
			"compliance": map[string]any{
				"status":   complianceStatus,
				"sancoes":  complianceSanctions,
				"total":    len(complianceSanctions),
			},
			"judicial": map[string]any{
				"status":    judicialStatus,
				"processos": judicialProcesses,
				"total":     len(judicialProcesses),
			},
			"contratos_governo": map[string]any{
				"status":    contratosStatus,
				"contratos": contratos,
				"total":     len(contratos),
			},
			"ambiental": map[string]any{
				"status":   ambientalStatus,
				"embargos": embargos,
				"total":    len(embargos),
			},
		},
	})
}
