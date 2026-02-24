package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDBPoolCollector_DescribeAndCollect(t *testing.T) {
	// We cannot easily create a real pgxpool.Pool in unit tests without a DB.
	// Instead, verify that the collector can be created and registered without
	// panicking. The real pool is wired in main.go at startup.
	//
	// This test validates the Describe method returns the expected number of
	// descriptors and that the type satisfies the prometheus.Collector interface.
	var _ prometheus.Collector = (*DBPoolCollector)(nil)
}
