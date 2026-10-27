package observability

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterDB(pool *pgxpool.Pool) {
	for name, read := range map[string]func() float64{
		"connections":          func() float64 { return float64(pool.Stat().TotalConns()) },
		"acquired_connections": func() float64 { return float64(pool.Stat().AcquiredConns()) },
		"idle_connections":     func() float64 { return float64(pool.Stat().IdleConns()) },
		"max_connections":      func() float64 { return float64(pool.Stat().MaxConns()) },
	} {
		_ = prometheus.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "ra_db_pool_" + name, Help: "Application PostgreSQL connection pool."}, read))
	}
	_ = prometheus.Register(prometheus.NewCounterFunc(prometheus.CounterOpts{Name: "ra_db_pool_acquire_wait_seconds_total", Help: "Cumulative database connection acquisition duration."}, func() float64 { return pool.Stat().AcquireDuration().Seconds() }))
	_ = prometheus.Register(prometheus.NewCounterFunc(prometheus.CounterOpts{Name: "ra_db_pool_canceled_acquires_total", Help: "Canceled database connection acquisitions."}, func() float64 { return float64(pool.Stat().CanceledAcquireCount()) }))
}
