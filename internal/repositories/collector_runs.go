package repositories

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CollectorRunRepository tracks the state of scheduled collectors.
type CollectorRunRepository struct {
	db *pgxpool.Pool
}

// NewCollectorRunRepository creates a new collector run repository.
func NewCollectorRunRepository(db *pgxpool.Pool) *CollectorRunRepository {
	return &CollectorRunRepository{db: db}
}

// RecordStart marks a collector as "running".
func (r *CollectorRunRepository) RecordStart(ctx context.Context, source string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO collector_runs (source, last_run_at, status, updated_at)
		VALUES ($1, NOW(), 'running', NOW())
		ON CONFLICT (source) DO UPDATE SET
			last_run_at = NOW(),
			status      = 'running',
			error_msg   = NULL,
			updated_at  = NOW()
	`, source)
	return err
}

// RecordSuccess marks a collector as "ok" and updates last_success + next_run_at.
func (r *CollectorRunRepository) RecordSuccess(ctx context.Context, source string, nextRun time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO collector_runs (source, last_run_at, last_success, next_run_at, status, updated_at)
		VALUES ($1, NOW(), NOW(), $2, 'ok', NOW())
		ON CONFLICT (source) DO UPDATE SET
			last_success = NOW(),
			next_run_at  = $2,
			status       = 'ok',
			error_msg    = NULL,
			updated_at   = NOW()
	`, source, nextRun)
	return err
}

// RecordError marks a collector as "error" with the error message.
func (r *CollectorRunRepository) RecordError(ctx context.Context, source, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO collector_runs (source, last_run_at, status, error_msg, updated_at)
		VALUES ($1, NOW(), 'error', $2, NOW())
		ON CONFLICT (source) DO UPDATE SET
			status    = 'error',
			error_msg = $2,
			updated_at = NOW()
	`, source, errMsg)
	return err
}
