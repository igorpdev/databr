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

// ufNomes maps Brazilian state codes to their full names.
var ufNomes = map[string]string{
	"AC": "Acre", "AL": "Alagoas", "AM": "Amazonas", "AP": "Amapá",
	"BA": "Bahia", "CE": "Ceará", "DF": "Distrito Federal", "ES": "Espírito Santo",
	"GO": "Goiás", "MA": "Maranhão", "MG": "Minas Gerais", "MS": "Mato Grosso do Sul",
	"MT": "Mato Grosso", "PA": "Pará", "PB": "Paraíba", "PE": "Pernambuco",
	"PI": "Piauí", "PR": "Paraná", "RJ": "Rio de Janeiro", "RN": "Rio Grande do Norte",
	"RO": "Rondônia", "RR": "Roraima", "RS": "Rio Grande do Sul", "SC": "Santa Catarina",
	"SE": "Sergipe", "SP": "São Paulo", "TO": "Tocantins",
}

// ibgePNADBaseURL is the base URL for the IBGE PNAD proxy. Override in tests.
var ibgePNADBaseURL = "https://servicodados.ibge.gov.br/api/v3/agregados"

// SetIBGEPNADBaseURL overrides the IBGE PNAD base URL (for testing).
func SetIBGEPNADBaseURL(url string) {
	if url == "" {
		ibgePNADBaseURL = "https://servicodados.ibge.gov.br/api/v3/agregados"
	} else {
		ibgePNADBaseURL = url
	}
}

// MercadoTrabalhoHandler builds a labor market analysis for a given Brazilian
// state (UF), combining employment statistics, economic indicators, and
// demographic data.
type MercadoTrabalhoHandler struct {
	store      SourceStore
	httpClient *http.Client
}

// NewMercadoTrabalhoHandler creates a labor market analysis handler.
func NewMercadoTrabalhoHandler(store SourceStore) *MercadoTrabalhoHandler {
	return &MercadoTrabalhoHandler{
		store:      store,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetHTTPClient overrides the HTTP client (for testing).
func (h *MercadoTrabalhoHandler) SetHTTPClient(c *http.Client) { h.httpClient = c }

// GetMercadoTrabalho handles GET /v1/mercado-trabalho/{uf}/analise.
func (h *MercadoTrabalhoHandler) GetMercadoTrabalho(w http.ResponseWriter, r *http.Request) {
	uf := strings.ToUpper(chi.URLParam(r, "uf"))
	if !isValidUF(uf) {
		jsonError(w, http.StatusBadRequest, "UF inválida. Use uma sigla válida (SP, RJ, MG, etc.)")
		return
	}

	ctx := r.Context()
	ufNome := ufNomes[uf]

	// Fetch IBGE demographic data via API (PNAD/population estimates).
	demografiaData := h.fetchDemografia(ctx, uf)

	// If store is nil, return partial data with demographics only.
	if h.store == nil {
		respond(w, r, domain.APIResponse{
			Source:    "mercado_trabalho",
			UpdatedAt: time.Now().UTC(),
			CostUSDC:  "0.010",
			Data: map[string]any{
				"estado": map[string]any{
					"uf":   uf,
					"nome": ufNome,
				},
				"demografia":            demografiaData,
				"emprego":               map[string]any{"disponivel": false, "motivo": "store not available"},
				"setores_principais":    []map[string]any{},
				"indicadores_economicos": map[string]any{"disponivel": false},
				"tendencia":             "indeterminada",
			},
		})
		return
	}

	// Parallel queries for employment and economic data.
	type queryResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		cagedRes queryResult
		raisRes  queryResult
		pibRes   queryResult
		ipcaRes  queryResult
		wg       sync.WaitGroup
	)

	wg.Add(4)
	go func() {
		defer wg.Done()
		cagedRes.records, cagedRes.err = h.store.FindLatestFiltered(ctx, "caged_emprego", "uf", uf)
	}()
	go func() {
		defer wg.Done()
		raisRes.records, raisRes.err = h.store.FindLatestFiltered(ctx, "rais_emprego", "uf", uf)
	}()
	go func() {
		defer wg.Done()
		pibRes.records, pibRes.err = h.store.FindLatest(ctx, "ibge_pib")
	}()
	go func() {
		defer wg.Done()
		ipcaRes.records, ipcaRes.err = h.store.FindLatest(ctx, "ibge_ipca")
	}()
	wg.Wait()

	// Build employment section.
	empregoData := map[string]any{"disponivel": false}
	setoresPrincipais := []map[string]any{}
	tendencia := "indeterminada"

	hasCaged := cagedRes.err == nil && len(cagedRes.records) > 0
	hasRais := raisRes.err == nil && len(raisRes.records) > 0

	if hasCaged || hasRais {
		empregoData = map[string]any{"disponivel": true}

		if hasCaged {
			admissoes := 0
			desligamentos := 0
			saldoTotal := 0
			setorMap := map[string]int{}

			for _, rec := range cagedRes.records {
				if adm, ok := toInt(rec.Data["admissoes"]); ok {
					admissoes += adm
				}
				if desl, ok := toInt(rec.Data["desligamentos"]); ok {
					desligamentos += desl
				}
				if saldo, ok := toInt(rec.Data["saldo"]); ok {
					saldoTotal += saldo
				}
				if setor, ok := rec.Data["setor"].(string); ok && setor != "" {
					setorMap[setor]++
				}
			}

			empregoData["caged"] = map[string]any{
				"admissoes":     admissoes,
				"desligamentos": desligamentos,
				"saldo":         saldoTotal,
				"registros":     len(cagedRes.records),
			}

			// Determine trend from CAGED balance.
			if saldoTotal > 0 {
				tendencia = "crescimento"
			} else if saldoTotal < 0 {
				tendencia = "declinio"
			} else {
				tendencia = "estavel"
			}

			// Build top sectors list.
			for setor, count := range setorMap {
				setoresPrincipais = append(setoresPrincipais, map[string]any{
					"setor":     setor,
					"registros": count,
				})
			}
		}

		if hasRais {
			empregos := 0
			for _, rec := range raisRes.records {
				if emp, ok := toInt(rec.Data["empregos"]); ok {
					empregos += emp
				}
			}
			empregoData["rais"] = map[string]any{
				"empregos_formais": empregos,
				"registros":        len(raisRes.records),
			}
		}
	}

	// Build economic indicators section.
	indicadoresEconomicos := map[string]any{"disponivel": false}
	if pibRes.err == nil && len(pibRes.records) > 0 {
		indicadoresEconomicos["disponivel"] = true
		indicadoresEconomicos["pib"] = pibRes.records[0].Data
	}
	if ipcaRes.err == nil && len(ipcaRes.records) > 0 {
		indicadoresEconomicos["disponivel"] = true
		indicadoresEconomicos["ipca"] = ipcaRes.records[0].Data
	}

	respond(w, r, domain.APIResponse{
		Source:    "mercado_trabalho",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  "0.010",
		Data: map[string]any{
			"estado": map[string]any{
				"uf":   uf,
				"nome": ufNome,
			},
			"demografia":             demografiaData,
			"emprego":                empregoData,
			"setores_principais":     setoresPrincipais,
			"indicadores_economicos": indicadoresEconomicos,
			"tendencia":              tendencia,
		},
	})
}

// fetchDemografia fetches state-level demographic data from IBGE SIDRA.
// Uses table 6579 (population estimates by UF).
func (h *MercadoTrabalhoHandler) fetchDemografia(ctx context.Context, uf string) map[string]any {
	// IBGE SIDRA: table 6579 = population estimates, with UF breakdown.
	// IBGE state codes mapped from UF.
	ibgeUFCode := ibgeUFCodes[uf]
	if ibgeUFCode == "" {
		return map[string]any{"disponivel": false, "motivo": "UF code not mapped"}
	}

	url := fmt.Sprintf("%s/6579/periodos/-1/variaveis/9324?localidades=N3[%s]", ibgePNADBaseURL, ibgeUFCode)

	var result []map[string]any
	if _, err := fetchJSON(ctx, h.httpClient, url, nil, &result); err != nil {
		return map[string]any{"disponivel": false, "motivo": "IBGE API unavailable"}
	}

	if len(result) == 0 {
		return map[string]any{"disponivel": false, "motivo": "no data returned"}
	}

	return map[string]any{
		"disponivel": true,
		"fonte":      "IBGE",
		"dados":      result,
	}
}

// ibgeUFCodes maps UF codes to IBGE numeric state codes.
var ibgeUFCodes = map[string]string{
	"RO": "11", "AC": "12", "AM": "13", "RR": "14", "PA": "15", "AP": "16",
	"TO": "17", "MA": "21", "PI": "22", "CE": "23", "RN": "24", "PB": "25",
	"PE": "26", "AL": "27", "SE": "28", "BA": "29", "MG": "31", "ES": "32",
	"RJ": "33", "SP": "35", "PR": "41", "SC": "42", "RS": "43", "MS": "50",
	"MT": "51", "GO": "52", "DF": "53",
}

// toInt extracts an int from an any value (handles float64 from JSON and int).
func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case int64:
		return int(val), true
	default:
		return 0, false
	}
}
