package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts API requests by endpoint and HTTP status code.
	// Only /v1/* and /mcp routes are tracked (infra endpoints excluded).
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "databr_requests_total",
		Help: "Total API requests by endpoint and status",
	}, []string{"endpoint", "status"})

	// RequestDuration measures request latency as a histogram.
	// Buckets span from 10ms to 10s to cover both fast 402 rejects and slow upstream proxies.
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "databr_request_duration_seconds",
		Help:    "Request duration in seconds",
		Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"endpoint"})

	// CacheHits counts Redis cache hits and misses per endpoint.
	// Label "hit" is "true" for cache hit, "false" for miss.
	CacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "databr_cache_hits_total",
		Help: "Cache hit/miss counts",
	}, []string{"endpoint", "hit"})
)
