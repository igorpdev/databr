package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// DBPoolCollector exports pgxpool.Stat() as Prometheus gauges.
// Register it with prometheus.MustRegister(NewDBPoolCollector(pool)).
type DBPoolCollector struct {
	pool *pgxpool.Pool

	acquireCount       *prometheus.Desc
	acquireDuration    *prometheus.Desc
	acquiredConns      *prometheus.Desc
	idleConns          *prometheus.Desc
	totalConns         *prometheus.Desc
	maxConns           *prometheus.Desc
	constructingConns  *prometheus.Desc
	emptyAcquireCount  *prometheus.Desc
}

// NewDBPoolCollector creates a collector that reads stats from the given pool.
func NewDBPoolCollector(pool *pgxpool.Pool) *DBPoolCollector {
	return &DBPoolCollector{
		pool: pool,
		acquireCount: prometheus.NewDesc(
			"databr_db_pool_acquire_count_total",
			"Cumulative count of successful acquires from the pool",
			nil, nil,
		),
		acquireDuration: prometheus.NewDesc(
			"databr_db_pool_acquire_duration_seconds_total",
			"Total duration of all successful acquires",
			nil, nil,
		),
		acquiredConns: prometheus.NewDesc(
			"databr_db_pool_acquired_conns",
			"Number of currently acquired connections",
			nil, nil,
		),
		idleConns: prometheus.NewDesc(
			"databr_db_pool_idle_conns",
			"Number of currently idle connections",
			nil, nil,
		),
		totalConns: prometheus.NewDesc(
			"databr_db_pool_total_conns",
			"Total number of connections in the pool",
			nil, nil,
		),
		maxConns: prometheus.NewDesc(
			"databr_db_pool_max_conns",
			"Maximum number of connections allowed",
			nil, nil,
		),
		constructingConns: prometheus.NewDesc(
			"databr_db_pool_constructing_conns",
			"Number of connections currently being constructed",
			nil, nil,
		),
		emptyAcquireCount: prometheus.NewDesc(
			"databr_db_pool_empty_acquire_count_total",
			"Cumulative count of acquires when pool was empty",
			nil, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *DBPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquireCount
	ch <- c.acquireDuration
	ch <- c.acquiredConns
	ch <- c.idleConns
	ch <- c.totalConns
	ch <- c.maxConns
	ch <- c.constructingConns
	ch <- c.emptyAcquireCount
}

// Collect implements prometheus.Collector.
func (c *DBPoolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.pool.Stat()

	ch <- prometheus.MustNewConstMetric(c.acquireCount, prometheus.CounterValue, float64(stat.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.acquireDuration, prometheus.CounterValue, stat.AcquireDuration().Seconds())
	ch <- prometheus.MustNewConstMetric(c.acquiredConns, prometheus.GaugeValue, float64(stat.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.idleConns, prometheus.GaugeValue, float64(stat.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.totalConns, prometheus.GaugeValue, float64(stat.TotalConns()))
	ch <- prometheus.MustNewConstMetric(c.maxConns, prometheus.GaugeValue, float64(stat.MaxConns()))
	ch <- prometheus.MustNewConstMetric(c.constructingConns, prometheus.GaugeValue, float64(stat.ConstructingConns()))
	ch <- prometheus.MustNewConstMetric(c.emptyAcquireCount, prometheus.CounterValue, float64(stat.EmptyAcquireCount()))
}
