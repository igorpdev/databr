package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// SourceStore is the minimal interface needed by BCB and Economia handlers.
type SourceStore interface {
	FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error)
	FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error)
	// FindLatestFiltered returns records for the given source where the JSONB
	// data field at jsonbKey contains jsonbValue (case-insensitive substring).
	// Useful for large datasets like ANEEL where in-memory filtering is impractical.
	FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error)
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
	respond(w, r, domain.APIResponse{
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

	respond(w, r, domain.APIResponse{
		Source:    match.Source,
		UpdatedAt: match.FetchedAt,
		CostUSDC:  "0.001",
		Data:      match.Data,
	})
}

// GetPIX handles GET /v1/bcb/pix/estatisticas.
func (h *BCBHandler) GetPIX(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_pix")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "PIX data not yet available")
		return
	}
	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetCredito handles GET /v1/bcb/credito.
func (h *BCBHandler) GetCredito(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_credito")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "Credit data not yet available")
		return
	}
	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetReservas handles GET /v1/bcb/reservas.
func (h *BCBHandler) GetReservas(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.FindLatest(r.Context(), "bcb_reservas")
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "Reserves data not yet available")
		return
	}
	rec := records[0]
	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// respond writes the API response, applying ?format=context if requested.
func respond(w http.ResponseWriter, r *http.Request, resp domain.APIResponse) {
	if r.URL.Query().Get("format") == "context" {
		b, err := json.Marshal(resp.Data)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to serialize context")
			return
		}
		resp.Context = fmt.Sprintf("[%s] %s", resp.Source, string(b))
		resp.Data = nil
		// Add $0.001 using integer milliUSDC to avoid float rounding
		if f, err := strconv.ParseFloat(resp.CostUSDC, 64); err == nil {
			millis := int64(math.Round(f * 1000))
			resp.CostUSDC = fmt.Sprintf("%.3f", float64(millis+1)/1000.0)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
