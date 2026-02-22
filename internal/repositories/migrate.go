package repositories

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies any pending SQL migration files from migrationsFS in lexicographic
// order. Progress is tracked in a schema_migrations table so the runner is idempotent.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsFS fs.FS) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// embed.FS.ReadDir returns entries in lexicographic order — deterministic.
	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version := entry.Name()

		var count int
		if err := pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM schema_migrations WHERE version=$1", version,
		).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if count > 0 {
			continue // already applied
		}

		sql, err := fs.ReadFile(migrationsFS, version)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("exec migration %s: %w", version, err)
		}
		if _, err := pool.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)", version,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		log.Printf("migration applied: %s", version)
	}
	return nil
}
