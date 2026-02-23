package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// MunicipioHandler aggregates demographic, environmental, and contract data
// for a given municipality.
type MunicipioHandler struct {
	store      SourceStore
	httpClient *http.Client
}

// NewMunicipioHandler creates a municipality profile handler.
func NewMunicipioHandler(store SourceStore) *MunicipioHandler {
	return &MunicipioHandler{
		store:      store,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetHTTPClient overrides the HTTP client (for testing).
func (h *MunicipioHandler) SetHTTPClient(c *http.Client) { h.httpClient = c }

// GetMunicipioPerfil handles GET /v1/municipios/{codigo}/perfil.
func (h *MunicipioHandler) GetMunicipioPerfil(w http.ResponseWriter, r *http.Request) {
	codigo := chi.URLParam(r, "codigo")
	if codigo == "" {
		jsonError(w, http.StatusBadRequest, "municipality code is required")
		return
	}

	ctx := r.Context()

	type queryResult struct {
		records []domain.SourceRecord
		err     error
	}

	var (
		populacaoRes   queryResult
		deterRes       queryResult
		licitacoesRes  queryResult
		wg             sync.WaitGroup
	)

	// DETER stores municipality names, not IBGE codes — resolve first
	municipioName := resolveIBGEToName(h.httpClient, codigo)

	wg.Add(3)
	go func() {
		defer wg.Done()
		populacaoRes.records, populacaoRes.err = h.store.FindLatestFiltered(ctx, "ibge_populacao", "codigo", codigo)
	}()
	go func() {
		defer wg.Done()
		deterRes.records, deterRes.err = h.store.FindLatestFiltered(ctx, "inpe_deter", "municipio", municipioName)
	}()
	go func() {
		defer wg.Done()
		licitacoesRes.records, licitacoesRes.err = h.store.FindLatestFiltered(ctx, "pncp_licitacoes", "municipio", codigo)
	}()
	wg.Wait()

	populacaoData := map[string]any{}
	if populacaoRes.err == nil && len(populacaoRes.records) > 0 {
		populacaoData = populacaoRes.records[0].Data
	}

	deterAlerts := []map[string]any{}
	if deterRes.err == nil {
		for _, rec := range deterRes.records {
			deterAlerts = append(deterAlerts, rec.Data)
		}
	}

	licitacoes := []map[string]any{}
	if licitacoesRes.err == nil {
		for _, rec := range licitacoesRes.records {
			licitacoes = append(licitacoes, rec.Data)
		}
	}

	respond(w, r, domain.APIResponse{
		Source:   "municipio_perfil",
		CostUSDC: x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"codigo":            codigo,
			"populacao":         populacaoData,
			"deter_alerts":      deterAlerts,
			"deter_alert_count": len(deterAlerts),
			"licitacoes":        licitacoes,
			"licitacoes_count":  len(licitacoes),
		},
	})
}
