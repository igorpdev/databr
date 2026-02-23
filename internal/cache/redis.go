// Package cache provides Redis-based caching for DataBR API responses.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps a Redis client for API response caching.
type Client struct {
	rdb *redis.Client
}

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = errors.New("cache miss")

// NewClient creates a Redis client from REDIS_URL environment variable.
func NewClient() (*Client, error) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379"
	}

	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("cache: parse redis URL: %w", err)
	}

	rdb := redis.NewClient(opts)
	return &Client{rdb: rdb}, nil
}

// NewClientFromURL creates a Redis client from an explicit URL (for testing).
func NewClientFromURL(url string) (*Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("cache: parse redis URL: %w", err)
	}
	return &Client{rdb: redis.NewClient(opts)}, nil
}

// Set stores a value in the cache with the given TTL.
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache: marshal %q: %w", key, err)
	}
	return c.rdb.Set(ctx, key, b, ttl).Err()
}

// Get retrieves a value from the cache and unmarshals it into dest.
// Returns ErrCacheMiss if the key does not exist.
func (c *Client) Get(ctx context.Context, key string, dest any) error {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrCacheMiss
	}
	if err != nil {
		return fmt.Errorf("cache: get %q: %w", key, err)
	}
	return json.Unmarshal(b, dest)
}

// TTL returns the remaining TTL for a key, or 0 if not found.
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.rdb.TTL(ctx, key).Result()
}

// Delete removes a key from the cache.
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Compile-time check: *Client implements Cacher.
var _ Cacher = (*Client)(nil)
