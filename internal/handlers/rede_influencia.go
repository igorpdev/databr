package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// RedeInfluenciaHandler maps the corporate network (shareholders and connected
// companies) for a given CNPJ.
type RedeInfluenciaHandler struct {
	cnpjFetcher CNPJFetcher
	store       SourceStore
}

// NewRedeInfluenciaHandler creates a rede influencia handler.
func NewRedeInfluenciaHandler(cnpjF CNPJFetcher, store SourceStore) *RedeInfluenciaHandler {
	return &RedeInfluenciaHandler{
		cnpjFetcher: cnpjF,
		store:       store,
	}
}

// socioInfo represents a partner/shareholder extracted from QSA data.
type socioInfo struct {
	Nome               string `json:"nome"`
	CPFOrCNPJ          string `json:"cpf_cnpj,omitempty"`
	Qualificacao       string `json:"qualificacao,omitempty"`
	DataEntrada        string `json:"data_entrada,omitempty"`
	FaixaEtaria        string `json:"faixa_etaria,omitempty"`
	RepresentanteLegal string `json:"representante_legal,omitempty"`
}

// connectedCompany represents a company found through a shared partner.
type connectedCompany struct {
	CNPJ        string `json:"cnpj"`
	RazaoSocial string `json:"razao_social,omitempty"`
	VinculoVia  string `json:"vinculo_via"` // name of the partner that connects
}

// GetRedeInfluencia handles GET /v1/rede/{cnpj}/influencia.
func (h *RedeInfluenciaHandler) GetRedeInfluencia(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)

	if !isValidCNPJ(normalized) {
		jsonError(w, http.StatusBadRequest, "CNPJ inválido — deve ter 14 dígitos válidos")
		return
	}

	ctx := r.Context()

	// Fetch central company data.
	companyRecords, err := h.cnpjFetcher.FetchByCNPJ(ctx, normalized)
	if err != nil {
		gatewayError(w, "rede_influencia", err)
		return
	}
	if len(companyRecords) == 0 {
		jsonError(w, http.StatusNotFound, "CNPJ não encontrado")
		return
	}

	companyData := companyRecords[0].Data

	// Extract QSA (quadro de socios e administradores).
	qsaRaw, ok := companyData["qsa"]
	if !ok || qsaRaw == nil {
		// Return response with empty network.
		respond(w, r, domain.APIResponse{
			Source:    "rede_influencia",
			UpdatedAt: time.Now().UTC(),
			CostUSDC:  "0.030",
			Data: map[string]any{
				"empresa_central": map[string]any{
					"cnpj":         normalized,
					"razao_social": companyData["razao_social"],
				},
				"socios":              []socioInfo{},
				"empresas_conectadas": []connectedCompany{},
				"estatisticas_rede": map[string]any{
					"total_socios":              0,
					"total_empresas_conectadas": 0,
					"profundidade":              1,
				},
			},
		})
		return
	}

	// Parse QSA entries into structured socio objects.
	socios := parseSocios(qsaRaw)

	// For each socio with a CNPJ or identifiable name, try to find connected companies.
	var (
		connected []connectedCompany
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	// Deduplicate: track CNPJs we have already seen to avoid listing the central company.
	seen := map[string]bool{normalized: true}

	for _, s := range socios {
		// Only search for connections if the partner has a CNPJ (i.e., is a legal entity).
		if s.CPFOrCNPJ == "" || len(s.CPFOrCNPJ) != 14 {
			continue
		}

		wg.Add(1)
		go func(socio socioInfo) {
			defer wg.Done()

			records, err := h.cnpjFetcher.FetchByCNPJ(ctx, socio.CPFOrCNPJ)
			if err != nil || len(records) == 0 {
				return
			}

			data := records[0].Data
			companyCNPJ := socio.CPFOrCNPJ
			razao := ""
			if rs, ok := data["razao_social"].(string); ok {
				razao = rs
			}

			mu.Lock()
			defer mu.Unlock()
			if !seen[companyCNPJ] {
				seen[companyCNPJ] = true
				connected = append(connected, connectedCompany{
					CNPJ:        companyCNPJ,
					RazaoSocial: razao,
					VinculoVia:  socio.Nome,
				})
			}
		}(s)
	}

	// Also search the store for companies that share the same partners.
	// Look for QSA entries in the store that reference partners from the central company.
	for _, s := range socios {
		if s.Nome == "" {
			continue
		}
		wg.Add(1)
		go func(socio socioInfo) {
			defer wg.Done()

			records, err := h.store.FindLatestFiltered(ctx, "cnpj", "qsa_nome", socio.Nome)
			if err != nil || len(records) == 0 {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, rec := range records {
				recCNPJ, _ := rec.Data["cnpj"].(string)
				if recCNPJ == "" || seen[recCNPJ] {
					continue
				}
				seen[recCNPJ] = true
				razao, _ := rec.Data["razao_social"].(string)
				connected = append(connected, connectedCompany{
					CNPJ:        recCNPJ,
					RazaoSocial: razao,
					VinculoVia:  socio.Nome,
				})
			}
		}(s)
	}

	wg.Wait()

	respond(w, r, domain.APIResponse{
		Source:    "rede_influencia",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.030",
		Data: map[string]any{
			"empresa_central": map[string]any{
				"cnpj":         normalized,
				"razao_social": companyData["razao_social"],
				"cnae_fiscal":  companyData["cnae_fiscal"],
				"uf":           companyData["uf"],
			},
			"socios":              socios,
			"empresas_conectadas": connected,
			"estatisticas_rede": map[string]any{
				"total_socios":              len(socios),
				"total_empresas_conectadas": len(connected),
				"profundidade":              1,
			},
		},
	})
}

// parseSocios extracts structured socio entries from the QSA field.
// The QSA field from minhareceita.org is typically []any where each element
// is a map[string]any with fields like "nome_socio", "cnpj_cpf_do_socio", etc.
func parseSocios(qsaRaw any) []socioInfo {
	qsaSlice, ok := qsaRaw.([]any)
	if !ok {
		return nil
	}

	socios := make([]socioInfo, 0, len(qsaSlice))
	for _, entry := range qsaSlice {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		s := socioInfo{}

		// minhareceita.org field names
		if v, ok := m["nome_socio"].(string); ok {
			s.Nome = v
		}
		if v, ok := m["cnpj_cpf_do_socio"].(string); ok {
			s.CPFOrCNPJ = reDigits.ReplaceAllString(v, "")
		}
		if v, ok := m["qualificacao_socio"].(string); ok {
			s.Qualificacao = v
		}
		if v, ok := m["data_entrada_sociedade"].(string); ok {
			s.DataEntrada = v
		}
		if v, ok := m["faixa_etaria"].(string); ok {
			s.FaixaEtaria = v
		}
		if v, ok := m["nome_representante_legal"].(string); ok {
			s.RepresentanteLegal = v
		}

		// Fallback: alternative field names used by some data sources
		if s.Nome == "" {
			if v, ok := m["nome"].(string); ok {
				s.Nome = v
			}
		}
		if s.CPFOrCNPJ == "" {
			if v, ok := m["cpf_cnpj"].(string); ok {
				s.CPFOrCNPJ = reDigits.ReplaceAllString(v, "")
			}
		}
		if s.Qualificacao == "" {
			if v, ok := m["qualificacao"].(string); ok {
				s.Qualificacao = v
			}
		}

		if s.Nome != "" {
			socios = append(socios, s)
		}
	}
	return socios
}
