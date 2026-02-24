package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/collectors/tributario"
	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// IBPTFetcher retrieves tax burden data for a given NCM/NBS code and UF.
type IBPTFetcher interface {
	FetchByNCM(ctx context.Context, codigo, uf string) ([]domain.SourceRecord, error)
}

// ICMSQuerier provides static ICMS rate lookups.
type ICMSQuerier interface {
	GetInternalRate(uf string) (*tributario.ICMSRate, error)
	GetInterstateRate(origem, destino string) (*tributario.InterstateRate, error)
	GetAllRates() []tributario.ICMSRate
}

// TributarioHandler handles /v1/tributario/* endpoints.
type TributarioHandler struct {
	ibpt IBPTFetcher
	icms ICMSQuerier
}

// NewTributarioHandler creates a new tributario handler.
func NewTributarioHandler(ibpt IBPTFetcher, icms ICMSQuerier) *TributarioHandler {
	return &TributarioHandler{ibpt: ibpt, icms: icms}
}

// GetNCMTributos handles GET /v1/tributario/ncm/{codigo}?uf=SP
func (h *TributarioHandler) GetNCMTributos(w http.ResponseWriter, r *http.Request) {
	codigo := chi.URLParam(r, "codigo")
	uf := r.URL.Query().Get("uf")

	if codigo == "" {
		jsonError(w, http.StatusBadRequest, "NCM/NBS code is required in URL path")
		return
	}
	if uf == "" {
		jsonError(w, http.StatusBadRequest, "query parameter 'uf' is required (e.g. ?uf=SP)")
		return
	}
	uf = strings.ToUpper(strings.TrimSpace(uf))
	if len(uf) != 2 || !tributario.ValidUFs[uf] {
		jsonError(w, http.StatusBadRequest, "invalid UF: must be a valid 2-letter Brazilian state code")
		return
	}

	records, err := h.ibpt.FetchByNCM(r.Context(), codigo, uf)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, http.StatusNotFound, "NCM/NBS code not found for the given UF")
			return
		}
		gatewayError(w, "ibpt", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "no tax data found")
		return
	}

	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      rec.Data,
	})
}

// GetICMS handles GET /v1/tributario/icms/{uf} and GET /v1/tributario/icms?origem=X&destino=Y
func (h *TributarioHandler) GetICMS(w http.ResponseWriter, r *http.Request) {
	uf := chi.URLParam(r, "uf")

	if uf != "" {
		// Single state internal rate.
		h.getICMSInterno(w, r, uf)
		return
	}

	// Check for interstate query params.
	origem := r.URL.Query().Get("origem")
	destino := r.URL.Query().Get("destino")

	if origem != "" && destino != "" {
		h.getICMSInterestadual(w, r, origem, destino)
		return
	}

	// No UF and no origem/destino: return all rates.
	h.getICMSAll(w, r)
}

func (h *TributarioHandler) getICMSInterno(w http.ResponseWriter, r *http.Request, uf string) {
	rate, err := h.icms.GetInternalRate(uf)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "icms_aliquotas",
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"uf":                  rate.UF,
			"nome":                rate.Nome,
			"aliquota_interna":    rate.AliquotaInterna,
			"fcp":                 rate.FCP,
			"aliquota_efetiva":    rate.AliquotaEfetiva,
			"regiao":              rate.Regiao,
			"grupo_interestadual": rate.GrupoInter,
		},
	})
}

func (h *TributarioHandler) getICMSInterestadual(w http.ResponseWriter, r *http.Request, origem, destino string) {
	result, err := h.icms.GetInterstateRate(origem, destino)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "icms_aliquotas",
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"origem":                  result.Origem,
			"destino":                 result.Destino,
			"aliquota_interestadual":  result.AliquotaInterestadual,
			"aliquota_importados":     result.AliquotaImportados,
			"difal":                   result.DIFAL,
			"regra":                   result.Regra,
		},
	})
}

func (h *TributarioHandler) getICMSAll(w http.ResponseWriter, r *http.Request) {
	rates := h.icms.GetAllRates()
	items := make([]map[string]any, 0, len(rates))
	for _, rate := range rates {
		items = append(items, map[string]any{
			"uf":                  rate.UF,
			"nome":                rate.Nome,
			"aliquota_interna":    rate.AliquotaInterna,
			"fcp":                 rate.FCP,
			"aliquota_efetiva":    rate.AliquotaEfetiva,
			"regiao":              rate.Regiao,
			"grupo_interestadual": rate.GrupoInter,
		})
	}

	respond(w, r, domain.APIResponse{
		Source:    "icms_aliquotas",
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"estados": items, "total": len(items)},
	})
}
