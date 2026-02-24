// Command purge runs the data retention policies once and exits.
// Usage: DATABASE_URL=... go run cmd/purge/main.go [--dry-run]
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/databr/api/internal/logging"
	"github.com/databr/api/internal/repositories"
	"github.com/joho/godotenv"
)

func main() {
	logging.Setup(nil)
	_ = godotenv.Load()

	dryRun := len(os.Args) > 1 && os.Args[1] == "--dry-run"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := repositories.NewPool(ctx)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if dryRun {
		slog.Info("=== DRY RUN — showing what WOULD be purged ===")
		policies := repositories.RetentionPolicies()
		for _, p := range policies {
			cutoff := time.Now().Add(-p.MaxAge)
			var count int64
			err := pool.QueryRow(ctx,
				"SELECT COUNT(*) FROM source_records WHERE source = $1 AND fetched_at < $2",
				p.Source, cutoff,
			).Scan(&count)
			if err != nil {
				slog.Error("query failed", "source", p.Source, "error", err)
				continue
			}
			if count > 0 {
				fmt.Printf("  %-25s %8d rows older than %v (cutoff: %s)\n",
					p.Source, count, p.MaxAge, cutoff.Format("2006-01-02"))
			}
		}

		// raw_data check
		cutoff := time.Now().Add(-repositories.RawDataMaxAge)
		var rawCount int64
		err := pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM source_records WHERE raw_data IS NOT NULL AND fetched_at < $1",
			cutoff,
		).Scan(&rawCount)
		if err == nil && rawCount > 0 {
			fmt.Printf("  %-25s %8d rows with raw_data older than %v\n",
				"[raw_data cleanup]", rawCount, repositories.RawDataMaxAge)
		}

		slog.Info("dry run complete — no data was modified")
		return
	}

	slog.Info("running retention purge on production database")
	purgeRepo := repositories.NewPurgeRepository(pool)
	if err := purgeRepo.RunRetention(ctx); err != nil {
		slog.Error("retention failed", "error", err)
		os.Exit(1)
	}
	slog.Info("retention complete")
}
