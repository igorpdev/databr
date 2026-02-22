package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// SourceStore is the minimal interface needed by BCB and Economia handlers.
type SourceStore interface {
	FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error)
	FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error)
}

// BCBHandler handles requests for /v1/bcb/*.
type BCBHandler struct {
	store SourceStore
}

// NewBCBHandler creates a BCB handler.
func NewBCBHandler(store SourceStore) *BCBHandler {
	return &BCBHandler{store: store}
}

// GetSelic handles GET /v1/bcb/selic.
func (h *BCBHandler) GetSelic(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_selic")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "Selic data not yet available")
		return
	}

	rec := records[0]
	respond(w, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetCambio handles GET /v1/bcb/cambio/{moeda}.
// Returns the most recent available PTAX rate for the given currency.
func (h *BCBHandler) GetCambio(w http.ResponseWriter, r *http.Request) {
	moeda := chi.URLParam(r, "moeda")

	records, err := h.store.FindLatest(r.Context(), "bcb_ptax")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Filter by currency: record key is "<MOEDA>_<DATE>"
	var match *domain.SourceRecord
	for i := range records {
		if m, ok := records[i].Data["moeda"].(string); ok && m == moeda {
			match = &records[i]
			break
		}
	}
	if match == nil {
		jsonError(w, http.StatusNotFound, "Exchange rate not found for "+moeda)
		return
	}

	respond(w, domain.APIResponse{
		Source:    match.Source,
		UpdatedAt: match.FetchedAt,
		CostUSDC:  "0.001",
		Data:      match.Data,
	})
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// respond writes a JSON API response.
func respond(w http.ResponseWriter, resp domain.APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
