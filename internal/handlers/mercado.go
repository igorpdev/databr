package handlers

import (
	"net/http"
	"strings"

	"github.com/databr/api/internal/collectors/cnpj"
	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// MercadoHandler handles /v1/mercado/* requests (B3 stocks, CVM funds).
type MercadoHandler struct {
	store SourceStore
}

// NewMercadoHandler creates a Mercado handler.
func NewMercadoHandler(store SourceStore) *MercadoHandler {
	return &MercadoHandler{store: store}
}

// GetAcoes handles GET /v1/mercado/acoes/{ticker}.
// Returns the last available B3 closing price for the given ticker.
func (h *MercadoHandler) GetAcoes(w http.ResponseWriter, r *http.Request) {
	ticker := chi.URLParam(r, "ticker")
	rec, err := h.store.FindOne(r.Context(), "b3_cotacoes", ticker)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if rec == nil {
		jsonError(w, http.StatusNotFound, "No quote found for ticker "+ticker)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.002",
		Data:      rec.Data,
	})
}

// GetFundos handles GET /v1/mercado/fundos/{cnpj}.
// Returns CVM fund data for the given CNPJ.
func (h *MercadoHandler) GetFundos(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := chi.URLParam(r, "cnpj")
	normalized := cnpj.NormalizeCNPJ(rawCNPJ)
	rec, err := h.store.FindOne(r.Context(), "cvm_fundos", normalized)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if rec == nil {
		jsonError(w, http.StatusNotFound, "Fundo não encontrado: "+normalized)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.005",
		Data:      rec.Data,
	})
}

// GetFatosRelevantes handles GET /v1/mercado/fatos-relevantes.
// Returns recent CVM "Fato Relevante" filings collected from the annual IPE CSV.
//
// Optional query parameter:
//   - ?cnpj=00000000000191  — filter filings by company CNPJ (substring match)
//
// Without a filter the most-recent records are returned.
// With ?cnpj= the store performs a JSONB-level substring filter on the cnpj field.
//
// Pricing: $0.002 USDC (+ $0.001 with ?format=context).
func (h *MercadoHandler) GetFatosRelevantes(w http.ResponseWriter, r *http.Request) {
	filterCNPJ := strings.TrimSpace(r.URL.Query().Get("cnpj"))

	var records []domain.SourceRecord
	var err error
	if filterCNPJ != "" {
		records, err = h.store.FindLatestFiltered(r.Context(), "cvm_fatos", "cnpj", filterCNPJ)
	} else {
		records, err = h.store.FindLatest(r.Context(), "cvm_fatos")
	}
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		if filterCNPJ != "" {
			jsonError(w, http.StatusNotFound, "Nenhum fato relevante encontrado para o CNPJ: "+filterCNPJ)
		} else {
			jsonError(w, http.StatusNotFound, "Fatos relevantes não disponíveis ainda")
		}
		return
	}

	items := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		items = append(items, map[string]any{
			"protocolo":  rec.RecordKey,
			"fetched_at": rec.FetchedAt,
			"data":       rec.Data,
		})
	}

	respond(w, r, domain.APIResponse{
		Source:    "cvm_fatos",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  "0.002",
		Data: map[string]any{
			"total":   len(records),
			"records": items,
		},
	})
}

// GetFatosById handles GET /v1/mercado/fatos-relevantes/{protocolo}.
// Returns a single CVM fato relevante identified by its Protocolo_Entrega.
//
// Pricing: $0.001 USDC (+ $0.001 with ?format=context).
func (h *MercadoHandler) GetFatosById(w http.ResponseWriter, r *http.Request) {
	protocolo := chi.URLParam(r, "protocolo")
	rec, err := h.store.FindOne(r.Context(), "cvm_fatos", protocolo)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if rec == nil {
		jsonError(w, http.StatusNotFound, "Fato relevante não encontrado: "+protocolo)
		return
	}
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}
