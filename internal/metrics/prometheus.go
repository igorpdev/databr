package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "databr_requests_total",
		Help: "Total API requests by endpoint and status",
	}, []string{"endpoint", "status"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "databr_request_duration_seconds",
		Help:    "Request duration in seconds",
		Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"endpoint"})

	CacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "databr_cache_hits_total",
		Help: "Cache hit/miss counts",
	}, []string{"endpoint", "hit"})

	PaymentsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "databr_payments_total",
		Help: "x402 payments by status",
	}, []string{"status", "amount_usdc"})
)
