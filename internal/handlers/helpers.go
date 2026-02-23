package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"

	"github.com/databr/api/internal/domain"
)

var reDigits = regexp.MustCompile(`\D`)

const maxResponseSize = 50 * 1024 * 1024 // 50 MB

// jsonError writes a JSON error response with the given HTTP status code.
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
		if f, err := strconv.ParseFloat(resp.CostUSDC, 64); err == nil {
			millis := int64(math.Round(f * 1000))
			resp.CostUSDC = fmt.Sprintf("%.3f", float64(millis+1)/1000.0)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// limitedReadAll reads up to maxResponseSize bytes from r.
// Prevents OOM from unexpectedly large upstream responses.
func limitedReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseSize))
}

// logUpstreamError logs the upstream error details server-side and returns a
// generic error message safe to show to clients.
func logUpstreamError(source string, statusCode int, body []byte) string {
	log.Printf("WARN: %s upstream error (HTTP %d): %s", source, statusCode, string(body))
	return "upstream service temporarily unavailable"
}

// ibgeBaseURL is the base URL for IBGE municipality API. Override in tests.
var ibgeBaseURL = "https://servicodados.ibge.gov.br/api/v1/localidades/municipios"

// SetIBGEBaseURL overrides the IBGE API base URL (for testing).
// Pass empty string to reset to default.
func SetIBGEBaseURL(url string) {
	if url == "" {
		ibgeBaseURL = "https://servicodados.ibge.gov.br/api/v1/localidades/municipios"
	} else {
		ibgeBaseURL = url
	}
}

// resolveIBGEToName converts an IBGE municipality code (e.g., "1302603") to
// the municipality name (e.g., "Manaus") via IBGE API. If the input doesn't
// look like an IBGE code (6-7 digit number) or the API call fails, the
// original input is returned unchanged.
func resolveIBGEToName(client *http.Client, code string) string {
	clean := reDigits.ReplaceAllString(code, "")
	if clean != code || len(code) < 6 {
		return code
	}
	url := fmt.Sprintf("%s/%s", ibgeBaseURL, code)
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return code
	}
	defer resp.Body.Close()
	var mun struct {
		Nome string `json:"nome"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mun); err != nil || mun.Nome == "" {
		return code
	}
	return mun.Nome
}
