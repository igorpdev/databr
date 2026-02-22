package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/databr/api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SourceRecordRepository handles database access for source_records.
type SourceRecordRepository struct {
	db *pgxpool.Pool
}

// NewSourceRecordRepository creates a new repository backed by the given pool.
func NewSourceRecordRepository(db *pgxpool.Pool) *SourceRecordRepository {
	return &SourceRecordRepository{db: db}
}

const upsertSQL = `
	INSERT INTO source_records (source, record_key, data, raw_data, fetched_at, valid_until)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (source, record_key) DO UPDATE SET
		data        = EXCLUDED.data,
		raw_data    = EXCLUDED.raw_data,
		fetched_at  = EXCLUDED.fetched_at,
		valid_until = EXCLUDED.valid_until
`

// upsertBatchSize is the number of records sent per pgx.SendBatch round-trip.
// Larger = fewer round-trips; smaller = less memory per batch.
const upsertBatchSize = 1000

// upsertLogEvery logs progress every N records during large upserts.
const upsertLogEvery = 10_000

// Upsert inserts or updates records using pgx.SendBatch to minimise round-trips.
// Records are processed in chunks of upsertBatchSize so memory stays bounded.
// Progress is logged every 10k records so long-running upserts are observable.
func (r *SourceRecordRepository) Upsert(ctx context.Context, records []domain.SourceRecord) error {
	if len(records) == 0 {
		return nil
	}

	total := len(records)
	source := records[0].Source
	log.Printf("[INFO] upsert %s: starting %d records", source, total)

	for i := 0; i < total; i += upsertBatchSize {
		end := i + upsertBatchSize
		if end > total {
			end = total
		}
		if err := r.upsertChunk(ctx, records[i:end]); err != nil {
			return err
		}
		if end%upsertLogEvery == 0 || end == total {
			log.Printf("[INFO] upsert %s: %d/%d records done", source, end, total)
		}
	}
	return nil
}

func (r *SourceRecordRepository) upsertChunk(ctx context.Context, records []domain.SourceRecord) error {
	batch := &pgx.Batch{}

	for _, rec := range records {
		dataJSON, err := json.Marshal(rec.Data)
		if err != nil {
			return fmt.Errorf("repositories: marshal data for %s/%s: %w", rec.Source, rec.RecordKey, err)
		}
		var rawJSON []byte
		if rec.RawData != nil {
			rawJSON, err = json.Marshal(rec.RawData)
			if err != nil {
				return fmt.Errorf("repositories: marshal raw_data for %s/%s: %w", rec.Source, rec.RecordKey, err)
			}
		}
		batch.Queue(upsertSQL, rec.Source, rec.RecordKey, dataJSON, rawJSON, rec.FetchedAt, rec.ValidUntil)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range records {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("repositories: batch upsert: %w", err)
		}
	}
	return br.Close()
}

// FindLatest returns the 100 most-recent records for the given source, ordered by fetched_at DESC.
func (r *SourceRecordRepository) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT source, record_key, data, raw_data, fetched_at, valid_until
		FROM source_records
		WHERE source = $1
		ORDER BY fetched_at DESC
		LIMIT 100
	`, source)
	if err != nil {
		return nil, fmt.Errorf("repositories: FindLatest %s: %w", source, err)
	}
	defer rows.Close()

	var records []domain.SourceRecord
	for rows.Next() {
		var rec domain.SourceRecord
		var dataJSON, rawJSON []byte
		var fetchedAt time.Time
		var validUntil *time.Time

		if err := rows.Scan(&rec.Source, &rec.RecordKey, &dataJSON, &rawJSON, &fetchedAt, &validUntil); err != nil {
			return nil, fmt.Errorf("repositories: FindLatest scan: %w", err)
		}
		rec.FetchedAt = fetchedAt
		rec.ValidUntil = validUntil
		if err := json.Unmarshal(dataJSON, &rec.Data); err != nil {
			continue
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// FindOne retrieves a single source record by (source, record_key).
// Returns (nil, nil) if not found.
func (r *SourceRecordRepository) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	var rec domain.SourceRecord
	var dataJSON, rawJSON []byte
	var fetchedAt time.Time
	var validUntil *time.Time

	err := r.db.QueryRow(ctx, `
		SELECT source, record_key, data, raw_data, fetched_at, valid_until
		FROM source_records
		WHERE source = $1 AND record_key = $2
	`, source, key).Scan(
		&rec.Source, &rec.RecordKey,
		&dataJSON, &rawJSON,
		&fetchedAt, &validUntil,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("repositories: find %s/%s: %w", source, key, err)
	}

	rec.FetchedAt = fetchedAt
	rec.ValidUntil = validUntil

	if err := json.Unmarshal(dataJSON, &rec.Data); err != nil {
		return nil, fmt.Errorf("repositories: unmarshal data: %w", err)
	}
	if rawJSON != nil {
		if err := json.Unmarshal(rawJSON, &rec.RawData); err != nil {
			return nil, fmt.Errorf("repositories: unmarshal raw_data: %w", err)
		}
	}

	return &rec, nil
}
