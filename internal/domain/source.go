// Package domain defines the core interfaces and types shared across the application.
// This package has zero external dependencies — only the Go standard library.
package domain

import (
	"context"
	"time"
)

// Version is the current API version, reported in /health.
const Version = "1.0.0"

// SourceRecord is the normalized, persisted representation of a data point from any source.
// All collectors produce []SourceRecord; all handlers read from it.
type SourceRecord struct {
	Source    string         `json:"source"`
	RecordKey string         `json:"record_key"`
	Data      map[string]any `json:"data"`
	RawData   map[string]any `json:"raw_data,omitempty"`
	FetchedAt time.Time      `json:"fetched_at"`
	ValidUntil *time.Time    `json:"valid_until,omitempty"`
}

// Collector is the interface every data source implements.
// Source returns a unique identifier (e.g. "bcb_selic", "ibge_ipca").
// Schedule returns a cron expression (e.g. "@daily", "0 6 * * *").
// Collect fetches data from the source and returns normalized records.
type Collector interface {
	Source() string
	Schedule() string
	Collect(ctx context.Context) ([]SourceRecord, error)
}

// APIResponse is the standard envelope for all /v1/* responses.
type APIResponse struct {
	Source          string         `json:"source"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Cached          bool           `json:"cached"`
	CacheAgeSeconds int            `json:"cache_age_seconds,omitempty"`
	CostUSDC        string         `json:"cost_usdc"`
	Data            map[string]any `json:"data,omitempty"`
	Context         string         `json:"context,omitempty"` // only when ?format=context
}
