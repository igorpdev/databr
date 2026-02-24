package repositories

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/databr/api/internal/domain"
)

// fakeRepo is a minimal in-memory SourceRecordRepository stand-in for tests.
type fakeRepo struct {
	mu      sync.Mutex
	records map[string][]domain.SourceRecord
	calls   int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{records: make(map[string][]domain.SourceRecord)}
}

func (f *fakeRepo) seed(source, key string, data map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records[source] = append(f.records[source], domain.SourceRecord{
		Source:    source,
		RecordKey: key,
		Data:     data,
		FetchedAt: time.Now(),
	})
}

func (f *fakeRepo) findLatest(_ context.Context, source string) ([]domain.SourceRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.records[source], nil
}

func (f *fakeRepo) findOne(_ context.Context, source, key string) (*domain.SourceRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	for _, r := range f.records[source] {
		if r.RecordKey == key {
			return &r, nil
		}
	}
	return nil, nil
}

// memCacheTestable wraps a fakeRepo with the same caching logic as
// CachedSourceRecordRepository but backed by the fake.
type memCacheTestable struct {
	fake  *fakeRepo
	ttl   time.Duration
	cache sync.Map
}

func newTestCache(fake *fakeRepo, ttl time.Duration) *memCacheTestable {
	return &memCacheTestable{fake: fake, ttl: ttl}
}

func (c *memCacheTestable) FindLatest(ctx context.Context, source string) ([]domain.SourceRecord, error) {
	key := source + ":latest"
	if entry, ok := c.cache.Load(key); ok {
		e := entry.(*memEntry)
		if time.Since(e.stored) < c.ttl {
			result := make([]domain.SourceRecord, len(e.records))
			copy(result, e.records)
			return result, nil
		}
		c.cache.Delete(key)
	}

	records, err := c.fake.findLatest(ctx, source)
	if err != nil {
		return nil, err
	}

	cached := make([]domain.SourceRecord, len(records))
	copy(cached, records)
	c.cache.Store(key, &memEntry{records: cached, stored: time.Now()})
	return records, nil
}

func (c *memCacheTestable) FindOne(ctx context.Context, source, key string) (*domain.SourceRecord, error) {
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

	rec, err := c.fake.findOne(ctx, source, key)
	if err != nil {
		return nil, err
	}

	c.cache.Store(cacheKey, &memEntry{single: rec, stored: time.Now()})
	return rec, nil
}

func TestMemCache_FindLatest_CachesResult(t *testing.T) {
	fake := newFakeRepo()
	fake.seed("bcb_selic", "2026-02-24", map[string]any{"valor": 13.25})

	cache := newTestCache(fake, 1*time.Second)
	ctx := context.Background()

	// First call hits the "database"
	r1, err := cache.FindLatest(ctx, "bcb_selic")
	if err != nil {
		t.Fatal(err)
	}
	if len(r1) != 1 {
		t.Fatalf("expected 1 record, got %d", len(r1))
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 DB call, got %d", fake.calls)
	}

	// Second call uses cache
	r2, err := cache.FindLatest(ctx, "bcb_selic")
	if err != nil {
		t.Fatal(err)
	}
	if len(r2) != 1 {
		t.Fatalf("expected 1 record, got %d", len(r2))
	}
	if fake.calls != 1 {
		t.Fatalf("expected still 1 DB call (cached), got %d", fake.calls)
	}
}

func TestMemCache_FindLatest_ExpiresAfterTTL(t *testing.T) {
	fake := newFakeRepo()
	fake.seed("bcb_selic", "2026-02-24", map[string]any{"valor": 13.25})

	cache := newTestCache(fake, 50*time.Millisecond)
	ctx := context.Background()

	_, _ = cache.FindLatest(ctx, "bcb_selic")
	if fake.calls != 1 {
		t.Fatalf("expected 1 DB call, got %d", fake.calls)
	}

	time.Sleep(60 * time.Millisecond)

	_, _ = cache.FindLatest(ctx, "bcb_selic")
	if fake.calls != 2 {
		t.Fatalf("expected 2 DB calls after TTL, got %d", fake.calls)
	}
}

func TestMemCache_FindOne_CachesResult(t *testing.T) {
	fake := newFakeRepo()
	fake.seed("cnpj", "12345678000190", map[string]any{"razao_social": "Test Corp"})

	cache := newTestCache(fake, 1*time.Second)
	ctx := context.Background()

	r1, err := cache.FindOne(ctx, "cnpj", "12345678000190")
	if err != nil {
		t.Fatal(err)
	}
	if r1 == nil {
		t.Fatal("expected non-nil record")
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 DB call, got %d", fake.calls)
	}

	r2, err := cache.FindOne(ctx, "cnpj", "12345678000190")
	if err != nil {
		t.Fatal(err)
	}
	if r2 == nil {
		t.Fatal("expected non-nil record")
	}
	if fake.calls != 1 {
		t.Fatalf("expected still 1 DB call (cached), got %d", fake.calls)
	}
}

func TestMemCache_FindOne_CachesNil(t *testing.T) {
	fake := newFakeRepo()
	cache := newTestCache(fake, 1*time.Second)
	ctx := context.Background()

	r1, err := cache.FindOne(ctx, "cnpj", "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if r1 != nil {
		t.Fatal("expected nil record")
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 DB call, got %d", fake.calls)
	}

	// Second call should use cache (even for nil result)
	r2, err := cache.FindOne(ctx, "cnpj", "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if r2 != nil {
		t.Fatal("expected nil record from cache")
	}
	if fake.calls != 1 {
		t.Fatalf("expected still 1 DB call (cached nil), got %d", fake.calls)
	}
}

func TestMemCache_ReturnsCopy(t *testing.T) {
	fake := newFakeRepo()
	fake.seed("bcb_selic", "2026-02-24", map[string]any{"valor": 13.25})

	cache := newTestCache(fake, 1*time.Second)
	ctx := context.Background()

	r1, _ := cache.FindLatest(ctx, "bcb_selic")
	r2, _ := cache.FindLatest(ctx, "bcb_selic")

	// Mutating r1 should not affect r2
	r1[0].Source = "mutated"
	if r2[0].Source == "mutated" {
		t.Fatal("cache returned same slice reference, not a copy")
	}
}
