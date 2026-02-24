package repositories

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RetentionPolicy defines how long records for a source should be kept.
type RetentionPolicy struct {
	Source    string
	MaxAge   time.Duration // Records older than this are purged
	Category string        // For logging: "time_series", "accumulating", "on_demand"
}

// RetentionPolicies returns the configured retention policies for all sources.
// Sources not listed here are SNAPSHOT (bounded, never purged).
func RetentionPolicies() []RetentionPolicy {
	const (
		day   = 24 * time.Hour
		week  = 7 * day
		month = 30 * day
		year  = 365 * day
	)

	return []RetentionPolicy{
		// 30 days — very high volume, only current snapshot matters
		{Source: "aneel_tarifas", MaxAge: 30 * day, Category: "accumulating"},
		{Source: "antt_rntrc", MaxAge: 30 * day, Category: "accumulating"},
		{Source: "anvisa_medicamentos", MaxAge: 30 * day, Category: "accumulating"},

		// 90 days — high volume, recent data is what's queried
		{Source: "cvm_cotas", MaxAge: 90 * day, Category: "accumulating"},
		{Source: "cvm_fundos", MaxAge: 90 * day, Category: "accumulating"},
		{Source: "cvm_fatos", MaxAge: 90 * day, Category: "accumulating"},
		{Source: "inpe_deter", MaxAge: 90 * day, Category: "accumulating"},
		{Source: "ibama_embargos", MaxAge: 90 * day, Category: "accumulating"},

		// 6 months — moderate volume, useful recent history
		{Source: "b3_cotacoes", MaxAge: 180 * day, Category: "time_series"},
		{Source: "ons_geracao", MaxAge: 180 * day, Category: "time_series"},
		{Source: "ons_carga", MaxAge: 180 * day, Category: "time_series"},
		{Source: "stf_decisoes", MaxAge: 180 * day, Category: "accumulating"},
		{Source: "stj_decisoes", MaxAge: 180 * day, Category: "accumulating"},
		{Source: "pncp_licitacoes", MaxAge: 180 * day, Category: "accumulating"},
		{Source: "tcu_acordaos", MaxAge: 180 * day, Category: "accumulating"},
		{Source: "comex_exportacoes", MaxAge: 180 * day, Category: "accumulating"},
		{Source: "comex_importacoes", MaxAge: 180 * day, Category: "accumulating"},
		{Source: "prf_acidentes", MaxAge: 180 * day, Category: "accumulating"},
		{Source: "caged_emprego", MaxAge: 180 * day, Category: "accumulating"},
		{Source: "anp_combustiveis", MaxAge: 180 * day, Category: "accumulating"},

		// 1 year — low volume time series, valuable historical data
		{Source: "bcb_selic", MaxAge: year, Category: "time_series"},
		{Source: "bcb_ptax", MaxAge: year, Category: "time_series"},
		{Source: "bcb_credito", MaxAge: year, Category: "time_series"},
		{Source: "bcb_reservas", MaxAge: year, Category: "time_series"},
		{Source: "ibge_ipca", MaxAge: year, Category: "time_series"},
		{Source: "ibge_pib", MaxAge: year, Category: "time_series"},
		{Source: "ipea_series", MaxAge: year, Category: "time_series"},

		// 7 days — on-demand cache, re-fetched from upstream if needed
		{Source: "cnpj", MaxAge: 7 * day, Category: "on_demand"},
		{Source: "cgu_compliance", MaxAge: 7 * day, Category: "on_demand"},
	}
}

// RawDataMaxAge is how long raw_data is kept before being cleared to NULL.
// Applies to ALL sources. raw_data is only useful for debugging.
const RawDataMaxAge = 7 * 24 * time.Hour

// purgeDeleteBatchSize limits each DELETE to avoid long-running transactions.
const purgeDeleteBatchSize = 5000

// PurgeRepository handles data retention and cleanup.
type PurgeRepository struct {
	db *pgxpool.Pool
}

// NewPurgeRepository creates a new purge repository.
func NewPurgeRepository(db *pgxpool.Pool) *PurgeRepository {
	return &PurgeRepository{db: db}
}

// PurgeSource deletes records older than maxAge for the given source.
// Deletes in batches to avoid long-running transactions.
// Returns the total number of rows deleted.
func (r *PurgeRepository) PurgeSource(ctx context.Context, source string, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	var totalDeleted int64

	for {
		res, err := r.db.Exec(ctx, `
			DELETE FROM source_records
			WHERE id IN (
				SELECT id FROM source_records
				WHERE source = $1 AND fetched_at < $2
				LIMIT $3
			)
		`, source, cutoff, purgeDeleteBatchSize)
		if err != nil {
			return totalDeleted, fmt.Errorf("purge %s: %w", source, err)
		}

		deleted := res.RowsAffected()
		totalDeleted += deleted

		if deleted < purgeDeleteBatchSize {
			break // No more rows to delete
		}
	}

	return totalDeleted, nil
}

// ClearOldRawData sets raw_data = NULL for records older than maxAge
// across ALL sources. This reclaims ~50% of storage without losing
// the normalized data field.
// Returns the total number of rows updated.
func (r *PurgeRepository) ClearOldRawData(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	var totalUpdated int64

	for {
		res, err := r.db.Exec(ctx, `
			UPDATE source_records
			SET raw_data = NULL
			WHERE id IN (
				SELECT id FROM source_records
				WHERE raw_data IS NOT NULL AND fetched_at < $1
				LIMIT $2
			)
		`, cutoff, purgeDeleteBatchSize)
		if err != nil {
			return totalUpdated, fmt.Errorf("clear raw_data: %w", err)
		}

		updated := res.RowsAffected()
		totalUpdated += updated

		if updated < purgeDeleteBatchSize {
			break
		}
	}

	return totalUpdated, nil
}

// RunRetention executes all retention policies and raw_data cleanup.
// Designed to be called from a cron job (e.g., weekly).
func (r *PurgeRepository) RunRetention(ctx context.Context) error {
	slog.Info("retention: starting purge cycle")
	start := time.Now()

	policies := RetentionPolicies()
	var totalPurged int64

	for _, p := range policies {
		deleted, err := r.PurgeSource(ctx, p.Source, p.MaxAge)
		if err != nil {
			slog.Error("retention: purge failed", "source", p.Source, "error", err)
			continue
		}
		totalPurged += deleted
		if deleted > 0 {
			slog.Info("retention: purged records",
				"source", p.Source,
				"deleted", deleted,
				"max_age", p.MaxAge.String(),
				"category", p.Category,
			)
		}
	}

	// Clear old raw_data across all sources
	cleared, err := r.ClearOldRawData(ctx, RawDataMaxAge)
	if err != nil {
		slog.Error("retention: clear raw_data failed", "error", err)
	} else if cleared > 0 {
		slog.Info("retention: cleared raw_data", "updated", cleared)
	}

	slog.Info("retention: purge cycle complete",
		"total_purged", totalPurged,
		"raw_data_cleared", cleared,
		"duration", time.Since(start).String(),
	)

	return nil
}
