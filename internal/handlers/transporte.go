package handlers

import (
	"net/http"
	"strings"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// TransporteHandler handles requests for /v1/transporte/*.
type TransporteHandler struct {
	store SourceStore
}

// NewTransporteHandler creates a TransporteHandler backed by the given SourceStore.
func NewTransporteHandler(store SourceStore) *TransporteHandler {
	return &TransporteHandler{store: store}
}

// GetAeronave handles GET /v1/transporte/aeronaves/{prefixo}.
// {prefixo} is the aircraft registration prefix (e.g. PR-ONY, PT-ZZZ).
// The prefix is normalized to uppercase before the lookup.
// Pricing: $0.001 USDC.
func (h *TransporteHandler) GetAeronave(w http.ResponseWriter, r *http.Request) {
	prefixo := strings.ToUpper(strings.TrimSpace(chi.URLParam(r, "prefixo")))

	rec, err := h.store.FindOne(r.Context(), "anac_rab", prefixo)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if rec == nil {
		jsonError(w, http.StatusNotFound, "aeronave não encontrada: "+prefixo)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetAeronaves handles GET /v1/transporte/aeronaves.
// Supports optional filters via query parameters:
//   - ?uf=SP        — filter by state of registration (SG_UF)
//   - ?fabricante=X — filter by manufacturer (NM_FABRICANTE)
//   - ?operador=X   — filter by operator name (NM_OPERADOR)
//
// Only the first provided filter is applied (priority: uf > fabricante > operador).
// Without any filter, returns a sample of 100 records.
// Pricing: $0.002 USDC.
func (h *TransporteHandler) GetAeronaves(w http.ResponseWriter, r *http.Request) {
	uf := strings.TrimSpace(r.URL.Query().Get("uf"))
	fabricante := strings.TrimSpace(r.URL.Query().Get("fabricante"))
	operador := strings.TrimSpace(r.URL.Query().Get("operador"))

	var records []domain.SourceRecord
	var err error

	switch {
	case uf != "":
		records, err = h.store.FindLatestFiltered(r.Context(), "anac_rab", "uf", uf)
	case fabricante != "":
		records, err = h.store.FindLatestFiltered(r.Context(), "anac_rab", "fabricante", fabricante)
	case operador != "":
		records, err = h.store.FindLatestFiltered(r.Context(), "anac_rab", "operador", operador)
	default:
		records, err = h.store.FindLatest(r.Context(), "anac_rab")
	}

	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		isFiltered := uf != "" || fabricante != "" || operador != ""
		if isFiltered {
			jsonError(w, http.StatusNotFound, "nenhuma aeronave encontrada com os filtros informados")
		} else {
			jsonError(w, http.StatusNotFound, "dados do RAB ainda não disponíveis")
		}
		return
	}

	items := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		items = append(items, map[string]any{
			"record_key": rec.RecordKey,
			"fetched_at": rec.FetchedAt,
			"data":       rec.Data,
		})
	}

	updatedAt := records[0].FetchedAt

	respond(w, r, domain.APIResponse{
		Source:    "anac_rab",
		UpdatedAt: updatedAt,
		CostUSDC:  "0.002",
		Data: map[string]any{
			"total":   len(records),
			"records": items,
		},
	})
}
