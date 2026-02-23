package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// CompeticaoHandler builds a competitive intelligence report for a given CNAE
// sector, aggregating B3-listed companies, PNCP procurement activity, and CVM
// fund exposure data.
type CompeticaoHandler struct {
	store      SourceStore
	httpClient *http.Client
}

// NewCompeticaoHandler creates a sector competition handler.
func NewCompeticaoHandler(store SourceStore) *CompeticaoHandler {
	return &CompeticaoHandler{
		store:      store,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetHTTPClient overrides the HTTP client (for testing).
func (h *CompeticaoHandler) SetHTTPClient(c *http.Client) { h.httpClient = c }

// ibgeCNAEBaseURL is the base URL for the IBGE CNAE API. Override in tests.
var ibgeCNAEBaseURL = "https://servicodados.ibge.gov.br/api/v2/cnae"

// SetIBGECNAEBaseURL overrides the IBGE CNAE base URL (for testing).
func SetIBGECNAEBaseURL(url string) {
	if url == "" {
		ibgeCNAEBaseURL = "https://servicodados.ibge.gov.br/api/v2/cnae"
	} else {
		ibgeCNAEBaseURL = url
	}
}

// GetCompeticao handles GET /v1/mercado/{cnae}/competicao.
func (h *CompeticaoHandler) GetCompeticao(w http.ResponseWriter, r *http.Request) {
	cnae := chi.URLParam(r, "cnae")
	if !isValidCNAE(cnae) {
		jsonError(w, http.StatusBadRequest, "CNAE code must be 2-7 digits")
		return
	}

	ctx := r.Context()

	// Fetch CNAE description from IBGE API.
	cnaeInfo := h.fetchCNAEInfo(ctx, cnae)

	// If store is nil, return partial data with CNAE info only.
	if h.store == nil {
		respond(w, r, domain.APIResponse{
			Source:    "competicao_setorial",
			UpdatedAt: time.Now().UTC(),
			CostUSDC:  "0.020",
			Data: map[string]any{
				"setor":             cnaeInfo,
				"empresas_listadas": map[string]any{"total": 0, "empresas": []map[string]any{}},
				"licitacoes_governo": map[string]any{"total": 0, "licitacoes": []map[string]any{}},
				"fundos_exposicao":  map[string]any{"total": 0, "fundos": []map[string]any{}},
				"indicadores": map[string]any{
					"hhi_estimado":          0,
					"atividade_mercado":     "indisponivel",
					"dados_parciais":        true,
					"motivo":                "store not available",
				},
			},
		})
		return
	}

	// Parallel queries for sector data.
	type queryResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		b3Res         queryResult
		pncpRes       queryResult
		cvmRes        queryResult
		wg            sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		b3Res.records, b3Res.err = h.store.FindLatest(ctx, "b3_cotacoes")
	}()
	go func() {
		defer wg.Done()
		pncpRes.records, pncpRes.err = h.store.FindLatest(ctx, "pncp_licitacoes")
	}()
	go func() {
		defer wg.Done()
		cvmRes.records, cvmRes.err = h.store.FindLatest(ctx, "cvm_fundos")
	}()
	wg.Wait()

	// Filter B3 companies by CNAE prefix if data includes it.
	cnaePrefix := cnae[:2] // Use the 2-digit division for broad matching
	empresasListadas := []map[string]any{}
	if b3Res.err == nil {
		for _, rec := range b3Res.records {
			recCNAE := extractStringOrNumber(rec.Data, "cnae")
			if recCNAE == "" {
				recCNAE = extractStringOrNumber(rec.Data, "cnae_fiscal")
			}
			if recCNAE != "" && strings.HasPrefix(recCNAE, cnaePrefix) {
				empresasListadas = append(empresasListadas, rec.Data)
			}
		}
	}

	// Filter PNCP procurement by sector activity.
	licitacoes := []map[string]any{}
	if pncpRes.err == nil {
		for _, rec := range pncpRes.records {
			desc, _ := rec.Data["descricao"].(string)
			objeto, _ := rec.Data["objeto"].(string)
			cnaeDesc, _ := cnaeInfo["descricao"].(string)
			// Match by CNAE description keywords in procurement description.
			if cnaeDesc != "" && (containsIgnoreCase(desc, cnaeDesc) || containsIgnoreCase(objeto, cnaeDesc)) {
				licitacoes = append(licitacoes, rec.Data)
			}
		}
	}

	// Filter CVM funds for sector exposure.
	fundos := []map[string]any{}
	if cvmRes.err == nil {
		for _, rec := range cvmRes.records {
			classe, _ := rec.Data["classe"].(string)
			nome, _ := rec.Data["nome"].(string)
			cnaeDesc, _ := cnaeInfo["descricao"].(string)
			if cnaeDesc != "" && (containsIgnoreCase(classe, cnaeDesc) || containsIgnoreCase(nome, cnaeDesc)) {
				fundos = append(fundos, rec.Data)
			}
		}
	}

	// Calculate estimated HHI (Herfindahl-Hirschman Index).
	hhi := 0
	atividadeMercado := "baixa"
	totalEmpresas := len(empresasListadas)
	if totalEmpresas > 0 {
		// Simple estimation: equal market share assumption
		sharePercent := 100.0 / float64(totalEmpresas)
		hhi = int(sharePercent * sharePercent * float64(totalEmpresas))
	}

	totalActivity := totalEmpresas + len(licitacoes) + len(fundos)
	if totalActivity >= 20 {
		atividadeMercado = "alta"
	} else if totalActivity >= 5 {
		atividadeMercado = "media"
	}

	respond(w, r, domain.APIResponse{
		Source:    "competicao_setorial",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.020",
		Data: map[string]any{
			"setor": cnaeInfo,
			"empresas_listadas": map[string]any{
				"total":    totalEmpresas,
				"empresas": empresasListadas,
			},
			"licitacoes_governo": map[string]any{
				"total":      len(licitacoes),
				"licitacoes": licitacoes,
			},
			"fundos_exposicao": map[string]any{
				"total":  len(fundos),
				"fundos": fundos,
			},
			"indicadores": map[string]any{
				"hhi_estimado":      hhi,
				"atividade_mercado": atividadeMercado,
			},
		},
	})
}

// fetchCNAEInfo fetches CNAE class/group/division info from the IBGE API.
func (h *CompeticaoHandler) fetchCNAEInfo(ctx context.Context, cnae string) map[string]any {
	// Try subclass first (7 digits), then class (5), group (3), division (2).
	endpoints := []struct {
		path  string
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
		if _, err := fetchJSON(ctx, h.httpClient, url, nil, &result); err == nil {
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

	// Fallback: return the raw code with no description.
	return map[string]any{
		"codigo":    cnae,
		"descricao": "Setor CNAE " + cnae,
	}
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	if s == "" || substr == "" {
		return false
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
