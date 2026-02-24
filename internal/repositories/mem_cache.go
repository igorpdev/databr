package repositories

import (
	"context"
	"sync"
	"time"

	"github.com/databr/api/internal/domain"
)

// DefaultMemCacheTTL is the default TTL for in-memory cached query results.
const DefaultMemCacheTTL = 30 * time.Second

type memEntry struct {
	records []domain.SourceRecord
	single  *domain.SourceRecord
	stored  time.Time
}

// CachedSourceRecordRepository wraps a SourceRecordRepository with an
// in-memory cache (sync.Map) to reduce database round-trips for repeated
// FindLatest and FindOne queries. Entries expire after TTL.
type CachedSourceRecordRepository struct {
	inner *SourceRecordRepository
	ttl   time.Duration
	cache sync.Map // key string → *memEntry
}

// NewCachedSourceRecordRepository creates a cached wrapper around the given repository.
func NewCachedSourceRecordRepository(inner *SourceRecordRepository, ttl time.Duration) *CachedSourceRecordRepository {
	return &CachedSourceRecordRepository{inner: inner, ttl: ttl}
}

// Upsert delegates to the inner repository and invalidates cached entries
// for the affected source.
func (c *CachedSourceRecordRepository) Upsert(ctx context.Context, records []domain.SourceRecord) error {
	err := c.inner.Upsert(ctx, records)
	if err == nil && len(records) > 0 {
		source := records[0].Source
		// Invalidate all cached entries for this source
		c.cache.Range(func(key, _ any) bool {
			if k, ok := key.(string); ok && len(k) > len(source) && k[:len(source)+1] == source+":" {
				c.cache.Delete(key)
			}
			return true
		})
	}
	return err
}

// FindLatest returns cached results if available and fresh, otherwise queries
// the database and caches the result.
func (c *CachedSourceRecordRepository) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	key := source + ":latest"
	if entry, ok := c.cache.Load(key); ok {
		e := entry.(*memEntry)
		if time.Since(e.stored) < c.ttl {
			// Return a copy to prevent mutation
			result := make([]domain.SourceRecord, len(e.records))
			copy(result, e.records)
			return result, nil
		}
		c.cache.Delete(key)
	}

	records, err := c.inner.FindLatest(ctx, source)
	if err != nil {
		return nil, err
	}

	cached := make([]domain.SourceRecord, len(records))
	copy(cached, records)
	c.cache.Store(key, &memEntry{records: cached, stored: time.Now()})
	return records, nil
}

// FindLatestFiltered delegates directly to the inner repository (not cached,
// since filtered queries have too many key combinations).
func (c *CachedSourceRecordRepository) FindLatestFiltered(ctx context.Context, source, jsonbKey, jsonbValue string) ([]domain.SourceRecord, error) {
	return c.inner.FindLatestFiltered(ctx, source, jsonbKey, jsonbValue)
}

// FindOne returns a cached result if available and fresh, otherwise queries
// the database and caches the result.
func (c *CachedSourceRecordRepository) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
	cacheKey := source + ":one:" + key
	if entry, ok := c.cache.Load(cacheKey); ok {
		e := entry.(*memEntry)
		if time.Since(e.stored) < c.ttl {
			if e.single == nil {
				return nil, nil
			}
			cp := *e.single
			return &cp, nil
		}
		c.cache.Delete(cacheKey)
	}

	rec, err := c.inner.FindOne(ctx, source, key)
	if err != nil {
		return nil, err
	}

	c.cache.Store(cacheKey, &memEntry{single: rec, stored: time.Now()})
	return rec, nil
}
