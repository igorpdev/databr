package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/databr/api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// TransportadoresHandler handles requests for /v1/transporte/transportadores/*.
// It uses SourceStore to query the "antt_rntrc" source populated by ANTTCollector.
type TransportadoresHandler struct {
	store SourceStore
}

// NewTransportadoresHandler creates a TransportadoresHandler backed by the given SourceStore.
func NewTransportadoresHandler(store SourceStore) *TransportadoresHandler {
	return &TransportadoresHandler{store: store}
}

// GetTransportador handles GET /v1/transporte/transportadores/{rntrc}.
//
// Path parameter {rntrc} is the RNTRC registration number (up to 9 digits).
// Leading zeros are added if necessary to normalise to the 9-digit padded format
// used as RecordKey in the database (e.g. "1" → "000000001").
//
// Pricing: $0.001 USDC (+ $0.001 with ?format=context).
func (h *TransportadoresHandler) GetTransportador(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "rntrc")
	rntrc := normaliseRNTRC(raw)
	if rntrc == "" {
		jsonError(w, http.StatusBadRequest, "rntrc inválido: "+raw)
		return
	}

	rec, err := h.store.FindOne(r.Context(), "antt_rntrc", rntrc)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if rec == nil {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("transportador não encontrado: %s", rntrc))
		return
	}

	respond(w, r, domain.APIResponse{
		Source:    rec.Source,
		UpdatedAt: rec.FetchedAt,
		CostUSDC:  "0.001",
		Data:      rec.Data,
	})
}

// GetTransportadoresByCNPJ handles GET /v1/transporte/transportadores?cnpj={cnpj}.
//
// The ?cnpj= query parameter accepts a CNPJ in any format (formatted or digits-only).
// Non-numeric characters are stripped before the DB lookup against the
// "cpf_cnpj_digits" JSONB key.
//
// Returns a list of matching carriers (there can be multiple registrations per CNPJ
// when a company has more than one RNTRC, e.g. suspended + active).
//
// Pricing: $0.002 USDC (+ $0.001 with ?format=context).
func (h *TransportadoresHandler) GetTransportadoresByCNPJ(w http.ResponseWriter, r *http.Request) {
	rawCNPJ := strings.TrimSpace(r.URL.Query().Get("cnpj"))
	if rawCNPJ == "" {
		jsonError(w, http.StatusBadRequest, "parâmetro ?cnpj= é obrigatório")
		return
	}

	digits := stripNonDigits(rawCNPJ)
	if digits == "" {
		jsonError(w, http.StatusBadRequest, "cnpj inválido: "+rawCNPJ)
		return
	}

	records, err := h.store.FindLatestFiltered(r.Context(), "antt_rntrc", "cpf_cnpj_digits", digits)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, "nenhum transportador encontrado para CNPJ: "+rawCNPJ)
		return
	}

	items := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		items = append(items, map[string]any{
			"rntrc":      rec.RecordKey,
			"fetched_at": rec.FetchedAt,
			"data":       rec.Data,
		})
	}

	updatedAt := records[0].FetchedAt
	respond(w, r, domain.APIResponse{
		Source:    "antt_rntrc",
		UpdatedAt: updatedAt,
		CostUSDC:  "0.002",
		Data: map[string]any{
			"total":   len(records),
			"records": items,
		},
	})
}

// normaliseRNTRC pads the raw RNTRC string with leading zeros to produce a 9-digit
// key matching the RecordKey format stored by the collector.
// Returns "" if rntrc contains non-digit characters or is empty.
func normaliseRNTRC(rntrc string) string {
	rntrc = strings.TrimSpace(rntrc)
	if rntrc == "" {
		return ""
	}
	for _, ch := range rntrc {
		if !unicode.IsDigit(ch) {
			return ""
		}
	}
	// Pad to 9 digits.
	for len(rntrc) < 9 {
		rntrc = "0" + rntrc
	}
	return rntrc
}

// stripNonDigits returns only the digit characters from s.
func stripNonDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
