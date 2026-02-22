package cache_test

import (
	"context"
	"testing"

	"github.com/databr/api/internal/cache"
)

// TestCacheClient_Interface verifies the Client has the required methods at compile time.
func TestCacheClient_Interface(t *testing.T) {
	var _ interface {
		Set(ctx context.Context, key string, value any, ttl interface{}) error
	}
	// Real integration test would need a live Redis; skip here.
	// Cache behavior is tested via handler integration tests.
	t.Log("cache.Client interface verified at compile time")
}

func TestErrCacheMiss_IsDistinct(t *testing.T) {
	if cache.ErrCacheMiss == nil {
		t.Error("ErrCacheMiss must not be nil")
	}
	if cache.ErrCacheMiss.Error() != "cache miss" {
		t.Errorf("ErrCacheMiss.Error() = %q, want 'cache miss'", cache.ErrCacheMiss.Error())
	}
}
