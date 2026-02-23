package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv" // used by parsePagination
	"time"

	"github.com/databr/api/internal/domain"
	x402pkg "github.com/databr/api/internal/x402"
)

var reDigits = regexp.MustCompile(`\D`)

const maxResponseSize = 50 * 1024 * 1024 // 50 MB

// defaultPagination holds the default and max page sizes for paginated endpoints.
const (
	defaultPageSize = 50
	maxPageSize     = 500
)

// jsonError writes a JSON error response with the given HTTP status code.
func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

// gatewayError logs the internal error details server-side and writes a generic
// error message to the client. Use this instead of exposing err.Error() directly.
func gatewayError(w http.ResponseWriter, source string, err error) {
	slog.Error("gateway error", "source", source, "error", err)
	jsonError(w, http.StatusBadGateway, "upstream service temporarily unavailable")
}

// internalError logs the error and writes a generic 500 to the client.
func internalError(w http.ResponseWriter, source string, err error) {
	slog.Error("internal error", "source", source, "error", err)
	jsonError(w, http.StatusInternalServerError, "internal error")
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
		resp.CostUSDC = x402pkg.AddContextPrice(resp.CostUSDC)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode API response", "error", err)
	}
}

// serveLatest is a helper for the common pattern: look up the latest record(s)
// for a source in the store and write the API response. Eliminates repetitive
// find → check error → check empty → respond boilerplate in store-backed handlers.
// Price is read from the request context (set by x402 middleware via PriceFromRequest).
func serveLatest(w http.ResponseWriter, r *http.Request, store SourceStore, source string) {
	records, err := store.FindLatest(r.Context(), source)
	if err != nil {
		gatewayError(w, source, err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, source+" data not yet available")
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

// serveLatestAll serves all records for a source as an array.
// Price is read from the request context (set by x402 middleware via PriceFromRequest).
func serveLatestAll(w http.ResponseWriter, r *http.Request, store SourceStore, source, dataKey string) {
	records, err := store.FindLatest(r.Context(), source)
	if err != nil {
		gatewayError(w, source, err)
		return
	}
	if len(records) == 0 {
		jsonError(w, http.StatusNotFound, source+" data not yet available")
		return
	}
	items := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		items = append(items, rec.Data)
	}
	respond(w, r, domain.APIResponse{
		Source:    source,
		UpdatedAt: records[0].FetchedAt,
		CostUSDC:  x402pkg.PriceFromRequest(r),
		Data:      map[string]any{dataKey: items, "total": len(items)},
	})
}

// parsePagination extracts limit and offset from query params with safe defaults.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultPageSize
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	return limit, offset
}

// paginateSlice applies limit/offset to a slice. Returns the paginated subset.
func paginateSlice[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

// fetchJSON is a helper for proxy handlers that fetch JSON from an upstream URL.
// It handles context, timeout, error logging, and JSON decoding in one place.
func fetchJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, dest any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = limitedReadAll(resp.Body) // drain body
		slog.Warn("upstream error", "url", url, "status", resp.StatusCode)
		return resp.StatusCode, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return resp.StatusCode, fmt.Errorf("decode response: %w", err)
	}
	return resp.StatusCode, nil
}

// RateLimitExceeded writes a 429 Too Many Requests response.
// Exported so it can be used by the rate limit handler in cmd/api/main.go.
func RateLimitExceeded(w http.ResponseWriter) {
	jsonError(w, http.StatusTooManyRequests, "rate limit exceeded, max 100 req/min")
}

// limitedReadAll reads up to maxResponseSize bytes from r.
// Prevents OOM from unexpectedly large upstream responses.
func limitedReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxResponseSize))
}

// logUpstreamError logs the upstream error details server-side and returns a
// generic error message safe to show to clients.
func logUpstreamError(source string, statusCode int, body []byte) string {
	slog.Warn("upstream error", "source", source, "status", statusCode)
	return "upstream service temporarily unavailable"
}

// newHTTPClient creates an HTTP client with the given timeout.
// Centralizes client creation to ensure consistent settings.
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
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
// the municipality name (e.g., "Manaus") via IBGE API.
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
