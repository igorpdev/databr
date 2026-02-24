package handlers

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/databr/api/internal/collectors/dou"
	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
	"github.com/go-chi/chi/v5"
)

// QDSearcher is the interface for on-demand Querido Diário search.
type QDSearcher interface {
	Search(ctx context.Context, params dou.SearchParams) ([]domain.SourceRecord, error)
	ListCities(ctx context.Context) ([]domain.SourceRecord, error)
	ListThemes(ctx context.Context) ([]string, error)
	SearchByTheme(ctx context.Context, theme string, params dou.SearchParams) ([]domain.SourceRecord, error)
}

// DOUHandler handles /v1/dou/* requests.
type DOUHandler struct {
	searcher QDSearcher
}

// NewDOUHandler creates a DOU handler backed by Querido Diário.
func NewDOUHandler(searcher QDSearcher) *DOUHandler {
	return &DOUHandler{searcher: searcher}
}

// GetBusca handles GET /v1/dou/busca?q=...&uf=SP&desde=2026-01-01&ate=2026-02-01
func (h *DOUHandler) GetBusca(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonError(w, http.StatusBadRequest, "query param 'q' is required")
		return
	}

	params := dou.SearchParams{
		Query: q,
		UF:    r.URL.Query().Get("uf"),
		Since: r.URL.Query().Get("desde"),
		Until: r.URL.Query().Get("ate"),
		Size:  20,
	}

	records, err := h.searcher.Search(r.Context(), params)
	if err != nil {
		gatewayError(w, "dou", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No results found for: "+q)
		return
	}

	items := make([]map[string]any, len(records))
	for i, rec := range records {
		items[i] = rec.Data
	}
	respond(w, r, domain.APIResponse{
		Source:    "querido_diario",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"resultados": items, "total": len(items), "query": q},
	})
}

// GetDiarios handles GET /v1/diarios/busca.
// Query params: q (required), municipio_ibge (IBGE municipality code), desde (YYYY-MM-DD), ate (YYYY-MM-DD).
func (h *DOUHandler) GetDiarios(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonError(w, http.StatusBadRequest, "query param 'q' is required")
		return
	}

	params := dou.SearchParams{
		Query:       q,
		TerritoryID: r.URL.Query().Get("municipio_ibge"),
		Since:       r.URL.Query().Get("desde"),
		Until:       r.URL.Query().Get("ate"),
		Size:        20,
	}

	records, err := h.searcher.Search(r.Context(), params)
	if err != nil {
		gatewayError(w, "dou", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "No results found for: "+q)
		return
	}

	items := make([]map[string]any, len(records))
	for i, rec := range records {
		items[i] = rec.Data
	}
	respond(w, r, domain.APIResponse{
		Source:    "querido_diario",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"resultados":    items,
			"total":         len(items),
			"query":         q,
			"municipio_ibge": params.TerritoryID,
		},
	})
}

// GetMunicipios handles GET /v1/diarios/municipios.
// Returns the list of municipalities with indexed official gazettes.
func (h *DOUHandler) GetMunicipios(w http.ResponseWriter, r *http.Request) {
	records, err := h.searcher.ListCities(r.Context())
	if err != nil {
		gatewayError(w, "querido_diario", err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "no municipalities found")
		return
	}

	items := make([]map[string]any, len(records))
	for i, rec := range records {
		items[i] = rec.Data
	}
	respond(w, r, domain.APIResponse{
		Source:    "querido_diario",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"municipios": items, "total": len(items)},
	})
}

// GetTemas handles GET /v1/diarios/temas.
// Returns the list of automatic classification themes.
func (h *DOUHandler) GetTemas(w http.ResponseWriter, r *http.Request) {
	themes, err := h.searcher.ListThemes(r.Context())
	if err != nil {
		gatewayError(w, "querido_diario", err)
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    "querido_diario",
		UpdatedAt: time.Now().UTC(),
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{"temas": themes, "total": len(themes)},
	})
}

// GetTema handles GET /v1/diarios/tema/{tema}.
// Searches official gazettes by classified theme.
func (h *DOUHandler) GetTema(w http.ResponseWriter, r *http.Request) {
	tema := chi.URLParam(r, "tema")
	if tema == "" {
		jsonError(w, http.StatusBadRequest, "tema is required")
		return
	}
	// URL-decode the theme name (e.g., "Pol%C3%ADticas%20Ambientais" → "Políticas Ambientais")
	decoded, err := url.PathUnescape(tema)
	if err != nil {
		decoded = tema
	}

	params := dou.SearchParams{
		TerritoryID: r.URL.Query().Get("municipio_ibge"),
		Since:       r.URL.Query().Get("desde"),
		Until:       r.URL.Query().Get("ate"),
		Size:        20,
	}

	records, searchErr := h.searcher.SearchByTheme(r.Context(), decoded, params)
	if searchErr != nil {
		gatewayError(w, "querido_diario", searchErr)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "no results found for theme: "+decoded)
		return
	}

	items := make([]map[string]any, len(records))
	for i, rec := range records {
		items[i] = rec.Data
	}
	respond(w, r, domain.APIResponse{
		Source:    "querido_diario",
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data: map[string]any{
			"resultados": items,
			"total":      len(items),
			"tema":       decoded,
		},
	})
}
