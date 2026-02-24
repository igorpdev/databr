package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/databr/api/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// QueryLogMiddleware logs each request's endpoint, duration, status code, and request ID.
// Sensitive path parameters (CNPJ, CPF, CEP) are masked in log output for LGPD compliance.
func QueryLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		duration := time.Since(start)

		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = r.URL.Path
		}

		// Use the route pattern (e.g. /v1/empresas/{cnpj}) not the actual path
		// to avoid logging PII like CNPJs and CPFs.
		reqID := middleware.GetReqID(r.Context())

		endpoint := maskPath(routePattern)

		// Only record Prometheus metrics for API routes — skip infra endpoints
		// (health, readyz, metrics, favicon) to avoid noise from probes and scrapers.
		if strings.HasPrefix(endpoint, "/v1/") || strings.HasPrefix(endpoint, "/mcp") {
			status := fmt.Sprintf("%d", ww.Status())
			metrics.RequestsTotal.WithLabelValues(endpoint, status).Inc()
			metrics.RequestDuration.WithLabelValues(endpoint).Observe(duration.Seconds())
		}

		slog.Info("query_log",
			"req_id", reqID,
			"endpoint", endpoint,
			"status", ww.Status(),
			"duration_ms", duration.Milliseconds(),
		)
	})
}

// maskPath replaces known sensitive path segments with masked versions.
// Only applied when the actual URL path (not route pattern) is used as fallback.
func maskPath(path string) string {
	// Route patterns already use {cnpj}, {doc}, etc. — safe to log as-is.
	// Only mask when the path contains actual values (no curly braces).
	if strings.Contains(path, "{") {
		return path
	}

	parts := strings.Split(path, "/")
	for i, part := range parts {
		// Mask anything that looks like a CNPJ (14 digits) or CPF (11 digits)
		digits := reDigits.ReplaceAllString(part, "")
		if len(digits) == 14 {
			parts[i] = digits[:4] + "**********"
		} else if len(digits) == 11 {
			parts[i] = digits[:3] + "********"
		}
	}
	return strings.Join(parts, "/")
}
