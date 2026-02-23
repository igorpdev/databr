package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// regulador describes a Brazilian regulatory agency.
type regulador struct {
	Nome    string `json:"nome"`
	Sigla   string `json:"sigla"`
	Papel   string `json:"papel"`
	Website string `json:"website"`
}

// cnaeReguladores maps CNAE division ranges to known regulatory agencies.
// The key is the 2-digit CNAE division; each maps to one or more agencies.
var cnaeReguladores = map[string][]regulador{
	// Mining (05-09)
	"05": {{Nome: "Agência Nacional de Mineração", Sigla: "ANM", Papel: "Regulação de atividades minerárias", Website: "https://www.gov.br/anm"}},
	"06": {{Nome: "Agência Nacional de Mineração", Sigla: "ANM", Papel: "Regulação de atividades minerárias", Website: "https://www.gov.br/anm"},
		{Nome: "Agência Nacional do Petróleo", Sigla: "ANP", Papel: "Regulação da indústria de petróleo e gás", Website: "https://www.gov.br/anp"}},
	"07": {{Nome: "Agência Nacional de Mineração", Sigla: "ANM", Papel: "Regulação de atividades minerárias", Website: "https://www.gov.br/anm"}},
	"08": {{Nome: "Agência Nacional de Mineração", Sigla: "ANM", Papel: "Regulação de atividades minerárias", Website: "https://www.gov.br/anm"}},
	"09": {{Nome: "Agência Nacional de Mineração", Sigla: "ANM", Papel: "Regulação de atividades minerárias", Website: "https://www.gov.br/anm"},
		{Nome: "Agência Nacional do Petróleo", Sigla: "ANP", Papel: "Regulação da indústria de petróleo e gás", Website: "https://www.gov.br/anp"}},
	// Manufacturing (10-33) — ANVISA + INMETRO
	"10": {{Nome: "Agência Nacional de Vigilância Sanitária", Sigla: "ANVISA", Papel: "Vigilância sanitária de alimentos", Website: "https://www.gov.br/anvisa"},
		{Nome: "Instituto Nacional de Metrologia", Sigla: "INMETRO", Papel: "Metrologia e qualidade industrial", Website: "https://www.gov.br/inmetro"}},
	"11": {{Nome: "Agência Nacional de Vigilância Sanitária", Sigla: "ANVISA", Papel: "Vigilância sanitária de bebidas", Website: "https://www.gov.br/anvisa"}},
	"12": {{Nome: "Agência Nacional de Vigilância Sanitária", Sigla: "ANVISA", Papel: "Vigilância sanitária de tabaco", Website: "https://www.gov.br/anvisa"}},
	"20": {{Nome: "Agência Nacional de Vigilância Sanitária", Sigla: "ANVISA", Papel: "Regulação de produtos químicos", Website: "https://www.gov.br/anvisa"},
		{Nome: "Instituto Nacional de Metrologia", Sigla: "INMETRO", Papel: "Metrologia e qualidade industrial", Website: "https://www.gov.br/inmetro"}},
	"21": {{Nome: "Agência Nacional de Vigilância Sanitária", Sigla: "ANVISA", Papel: "Regulação de medicamentos e cosméticos", Website: "https://www.gov.br/anvisa"}},
	// Electricity (35)
	"35": {{Nome: "Agência Nacional de Energia Elétrica", Sigla: "ANEEL", Papel: "Regulação do setor elétrico", Website: "https://www.aneel.gov.br"},
		{Nome: "Operador Nacional do Sistema Elétrico", Sigla: "ONS", Papel: "Coordenação da operação do sistema elétrico", Website: "https://www.ons.org.br"}},
	// Water and sanitation (36-39)
	"36": {{Nome: "Agência Nacional de Águas e Saneamento Básico", Sigla: "ANA", Papel: "Regulação do uso de recursos hídricos", Website: "https://www.gov.br/ana"}},
	"37": {{Nome: "Agência Nacional de Águas e Saneamento Básico", Sigla: "ANA", Papel: "Regulação de saneamento básico", Website: "https://www.gov.br/ana"}},
	// Transport (49-53)
	"49": {{Nome: "Agência Nacional de Transportes Terrestres", Sigla: "ANTT", Papel: "Regulação de transportes terrestres", Website: "https://www.gov.br/antt"}},
	"50": {{Nome: "Agência Nacional de Transportes Aquaviários", Sigla: "ANTAQ", Papel: "Regulação de transportes aquaviários", Website: "https://www.gov.br/antaq"}},
	"51": {{Nome: "Agência Nacional de Aviação Civil", Sigla: "ANAC", Papel: "Regulação da aviação civil", Website: "https://www.gov.br/anac"}},
	"52": {{Nome: "Agência Nacional de Transportes Terrestres", Sigla: "ANTT", Papel: "Regulação de armazenamento e transporte", Website: "https://www.gov.br/antt"}},
	"53": {{Nome: "Agência Nacional de Telecomunicações", Sigla: "ANATEL", Papel: "Regulação de serviços postais", Website: "https://www.gov.br/anatel"}},
	// Telecom (61)
	"61": {{Nome: "Agência Nacional de Telecomunicações", Sigla: "ANATEL", Papel: "Regulação de telecomunicações", Website: "https://www.gov.br/anatel"}},
	// Financial (64-66)
	"64": {{Nome: "Banco Central do Brasil", Sigla: "BCB", Papel: "Regulação e supervisão bancária", Website: "https://www.bcb.gov.br"},
		{Nome: "Comissão de Valores Mobiliários", Sigla: "CVM", Papel: "Regulação do mercado de capitais", Website: "https://www.gov.br/cvm"},
		{Nome: "Superintendência de Seguros Privados", Sigla: "SUSEP", Papel: "Regulação de seguros e previdência complementar", Website: "https://www.gov.br/susep"}},
	"65": {{Nome: "Superintendência de Seguros Privados", Sigla: "SUSEP", Papel: "Regulação de seguros", Website: "https://www.gov.br/susep"},
		{Nome: "Agência Nacional de Saúde Suplementar", Sigla: "ANS", Papel: "Regulação de planos de saúde", Website: "https://www.gov.br/ans"}},
	"66": {{Nome: "Comissão de Valores Mobiliários", Sigla: "CVM", Papel: "Regulação do mercado de capitais", Website: "https://www.gov.br/cvm"},
		{Nome: "Banco Central do Brasil", Sigla: "BCB", Papel: "Regulação de intermediação financeira", Website: "https://www.bcb.gov.br"}},
	// Public administration (84)
	"84": {{Nome: "Controladoria-Geral da União", Sigla: "CGU", Papel: "Controle interno e combate à corrupção", Website: "https://www.gov.br/cgu"},
		{Nome: "Tribunal de Contas da União", Sigla: "TCU", Papel: "Fiscalização de contas públicas", Website: "https://www.tcu.gov.br"}},
	// Health (86-88)
	"86": {{Nome: "Agência Nacional de Saúde Suplementar", Sigla: "ANS", Papel: "Regulação de planos de saúde", Website: "https://www.gov.br/ans"},
		{Nome: "Agência Nacional de Vigilância Sanitária", Sigla: "ANVISA", Papel: "Vigilância sanitária", Website: "https://www.gov.br/anvisa"}},
	"87": {{Nome: "Agência Nacional de Vigilância Sanitária", Sigla: "ANVISA", Papel: "Regulação de serviços de saúde", Website: "https://www.gov.br/anvisa"}},
	"88": {{Nome: "Agência Nacional de Vigilância Sanitária", Sigla: "ANVISA", Papel: "Regulação de serviços sociais", Website: "https://www.gov.br/anvisa"}},
}

// RegulacaoSetorHandler builds a regulatory landscape report for a given CNAE
// sector, mapping it to regulatory agencies and compliance requirements.
type RegulacaoSetorHandler struct {
	store      SourceStore
	httpClient *http.Client
}

// NewRegulacaoSetorHandler creates a sector regulation handler.
func NewRegulacaoSetorHandler(store SourceStore) *RegulacaoSetorHandler {
	return &RegulacaoSetorHandler{
		store:      store,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetHTTPClient overrides the HTTP client (for testing).
func (h *RegulacaoSetorHandler) SetHTTPClient(c *http.Client) { h.httpClient = c }

// GetRegulacaoSetor handles GET /v1/setor/{cnae}/regulacao.
func (h *RegulacaoSetorHandler) GetRegulacaoSetor(w http.ResponseWriter, r *http.Request) {
	cnae := chi.URLParam(r, "cnae")
	if !isValidCNAE(cnae) {
		jsonError(w, http.StatusBadRequest, "CNAE code must be 2-7 digits")
		return
	}

	ctx := r.Context()

	// Fetch CNAE description from IBGE API.
	cnaeInfo := fetchCNAEDescription(ctx, h.httpClient, cnae)

	// Map CNAE to regulatory agencies using the 2-digit division prefix.
	division := cnae[:2]
	reguladores := cnaeReguladores[division]

	reguladoresData := make([]map[string]any, 0, len(reguladores))
	for _, reg := range reguladores {
		reguladoresData = append(reguladoresData, map[string]any{
			"nome":    reg.Nome,
			"sigla":   reg.Sigla,
			"papel":   reg.Papel,
			"website": reg.Website,
		})
	}

	// Build compliance requirements based on sector.
	requisitos := buildComplianceRequirements(division)

	// Determine regulation level.
	nivelRegulacao := classifyRegulationLevel(division, len(reguladores))

	// If store is nil, return without legislation data.
	if h.store == nil {
		respond(w, r, domain.APIResponse{
			Source:    "regulacao_setorial",
			UpdatedAt: time.Now().UTC(),
			CostUSDC:  "0.015",
			Data: map[string]any{
				"cnae":                   cnaeInfo,
				"reguladores_principais": reguladoresData,
				"requisitos_compliance":  requisitos,
				"legislacao_recente":     []map[string]any{},
				"nivel_regulacao":        nivelRegulacao,
			},
		})
		return
	}

	// Parallel queries for legislation and compliance data.
	type queryResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		legislacaoRes  queryResult
		complianceRes  queryResult
		wg             sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		legislacaoRes.records, legislacaoRes.err = h.store.FindLatest(ctx, "camara_deputados")
	}()
	go func() {
		defer wg.Done()
		complianceRes.records, complianceRes.err = h.store.FindLatest(ctx, "cgu_compliance")
	}()
	wg.Wait()

	// Filter legislation for sector relevance.
	legislacaoRecente := []map[string]any{}
	if legislacaoRes.err == nil {
		cnaeDesc, _ := cnaeInfo["descricao"].(string)
		for _, rec := range legislacaoRes.records {
			ementa, _ := rec.Data["ementa"].(string)
			assunto, _ := rec.Data["assunto"].(string)
			if cnaeDesc != "" && (containsIgnoreCase(ementa, cnaeDesc) || containsIgnoreCase(assunto, cnaeDesc)) {
				legislacaoRecente = append(legislacaoRecente, rec.Data)
			}
		}
	}

	// Add compliance sanctions count if available.
	if complianceRes.err == nil && len(complianceRes.records) > 0 {
		requisitos = append(requisitos, map[string]any{
			"tipo":      "monitoramento",
			"descricao": fmt.Sprintf("Verificar listas de sanções (CEIS/CNEP) — %d registros ativos no sistema", len(complianceRes.records)),
		})
	}

	respond(w, r, domain.APIResponse{
		Source:    "regulacao_setorial",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.015",
		Data: map[string]any{
			"cnae":                   cnaeInfo,
			"reguladores_principais": reguladoresData,
			"requisitos_compliance":  requisitos,
			"legislacao_recente":     legislacaoRecente,
			"nivel_regulacao":        nivelRegulacao,
		},
	})
}

// fetchCNAEDescription fetches the CNAE description from the IBGE API.
func fetchCNAEDescription(ctx context.Context, client *http.Client, cnae string) map[string]any {
	endpoints := []struct {
		path   string
		minLen int
	}{
		{fmt.Sprintf("/subclasses/%s", cnae), 7},
		{fmt.Sprintf("/classes/%s", cnae), 5},
		{fmt.Sprintf("/grupos/%s", cnae), 3},
		{fmt.Sprintf("/divisoes/%s", cnae), 2},
	}

	for _, ep := range endpoints {
		if len(cnae) < ep.minLen {
			continue
		}
		url := ibgeCNAEBaseURL + ep.path
		var result map[string]any
		if _, err := fetchJSON(ctx, client, url, nil, &result); err == nil {
			desc, _ := result["descricao"].(string)
			id, _ := result["id"].(string)
			if id == "" {
				id = cnae
			}
			return map[string]any{
				"codigo":    id,
				"descricao": desc,
			}
		}
	}

	return map[string]any{
		"codigo":    cnae,
		"descricao": "Setor CNAE " + cnae,
	}
}

// buildComplianceRequirements returns a list of typical compliance requirements
// for a given CNAE division.
func buildComplianceRequirements(division string) []map[string]any {
	reqs := []map[string]any{
		{"tipo": "geral", "descricao": "Cadastro Nacional de Pessoa Jurídica (CNPJ) ativo na Receita Federal"},
		{"tipo": "geral", "descricao": "Alvará de funcionamento municipal"},
	}

	divNum, _ := strconv.Atoi(division)

	switch {
	case divNum >= 5 && divNum <= 9:
		reqs = append(reqs,
			map[string]any{"tipo": "ambiental", "descricao": "Licenciamento ambiental (IBAMA/órgão estadual)"},
			map[string]any{"tipo": "setorial", "descricao": "Autorização de lavra (ANM)"},
			map[string]any{"tipo": "seguranca", "descricao": "Plano de Fechamento de Mina"},
		)
	case divNum >= 10 && divNum <= 33:
		reqs = append(reqs,
			map[string]any{"tipo": "sanitario", "descricao": "Registro ANVISA para produtos industrializados (quando aplicável)"},
			map[string]any{"tipo": "qualidade", "descricao": "Certificação INMETRO (quando aplicável)"},
			map[string]any{"tipo": "ambiental", "descricao": "Licenciamento ambiental para atividades industriais"},
		)
	case division == "35":
		reqs = append(reqs,
			map[string]any{"tipo": "setorial", "descricao": "Autorização ANEEL para geração, transmissão ou distribuição"},
			map[string]any{"tipo": "ambiental", "descricao": "Licenciamento ambiental para empreendimentos energéticos"},
			map[string]any{"tipo": "operacional", "descricao": "Conformidade com procedimentos de rede do ONS"},
		)
	case divNum >= 49 && divNum <= 53:
		reqs = append(reqs,
			map[string]any{"tipo": "setorial", "descricao": "Autorização de operação (ANTT/ANAC/ANTAQ conforme modal)"},
			map[string]any{"tipo": "seguranca", "descricao": "Certificados de segurança operacional"},
		)
	case division == "61":
		reqs = append(reqs,
			map[string]any{"tipo": "setorial", "descricao": "Outorga de serviço de telecomunicações (ANATEL)"},
			map[string]any{"tipo": "tecnico", "descricao": "Homologação de equipamentos (ANATEL)"},
		)
	case divNum >= 64 && divNum <= 66:
		reqs = append(reqs,
			map[string]any{"tipo": "setorial", "descricao": "Autorização de funcionamento BCB/CVM/SUSEP"},
			map[string]any{"tipo": "compliance", "descricao": "Programa de Prevenção à Lavagem de Dinheiro (PLD/FT)"},
			map[string]any{"tipo": "reporte", "descricao": "Relatórios periódicos ao regulador (BACEN DLO, CVM)"},
		)
	case division == "84":
		reqs = append(reqs,
			map[string]any{"tipo": "transparencia", "descricao": "Lei de Acesso à Informação (LAI)"},
			map[string]any{"tipo": "compliance", "descricao": "Lei de Responsabilidade Fiscal"},
			map[string]any{"tipo": "licitacao", "descricao": "Lei de Licitações (Lei 14.133/2021)"},
		)
	case divNum >= 86 && divNum <= 88:
		reqs = append(reqs,
			map[string]any{"tipo": "sanitario", "descricao": "Alvará sanitário (ANVISA/Vigilância Sanitária)"},
			map[string]any{"tipo": "setorial", "descricao": "Registro de operadora ANS (planos de saúde)"},
			map[string]any{"tipo": "profissional", "descricao": "Registro profissional nos conselhos de classe (CRM, COREN, etc.)"},
		)
	}

	return reqs
}

// classifyRegulationLevel returns "low", "medium", or "high" based on the CNAE
// division and the number of regulatory agencies.
func classifyRegulationLevel(division string, numReguladores int) string {
	divNum, _ := strconv.Atoi(division)

	// Heavily regulated sectors.
	switch {
	case divNum >= 64 && divNum <= 66: // Financial
		return "alto"
	case division == "35": // Electricity
		return "alto"
	case divNum >= 86 && divNum <= 88: // Health
		return "alto"
	case division == "61": // Telecom
		return "alto"
	case divNum >= 5 && divNum <= 9: // Mining
		return "alto"
	case division == "84": // Public admin
		return "alto"
	case divNum >= 49 && divNum <= 53: // Transport
		return "medio"
	case divNum >= 10 && divNum <= 33: // Manufacturing
		return "medio"
	}

	if numReguladores >= 2 {
		return "medio"
	}

	return "baixo"
}
