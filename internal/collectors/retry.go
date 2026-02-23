// Package collectors provides shared utilities for all data source collectors.
package collectors

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// DoWithRetry executes an HTTP request with exponential backoff + jitter.
// Retries on 429 (rate limited) and 5xx (server error) responses.
// The caller is responsible for closing the response body on success.
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			jitter := time.Duration(rand.Int63n(int64(time.Second)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff + jitter):
			}
		}

		// Clone the request for retry (body must be re-readable or nil for GET).
		cloned := req.Clone(ctx)
		resp, err := client.Do(cloned)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}
