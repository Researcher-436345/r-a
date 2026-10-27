package analytics

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var aggregationErrors = promauto.NewCounter(prometheus.CounterOpts{Name: "ra_product_aggregation_errors_total", Help: "Failed product aggregation cycles; the last good snapshot stays available."})

var specs = map[string][]string{
	"tracking_started_timestamp_seconds": {}, "aggregation_last_success_timestamp_seconds": {},
	"registered_users": {}, "active_users": {"window"}, "value_users": {"window"},
	"activation_users": {"state"}, "time_to_first_value_seconds": {"quantile"},
	"retention_users": {"period", "state"}, "project_return_projects": {"period", "state"},
	"project_activation_projects": {"state"}, "feature_users": {"feature"}, "feature_uses": {"feature"},
	"feature_usage_quantile": {"feature", "quantile"}, "action_results": {"feature", "outcome"},
	"search_funnel_actions": {"feature", "stage"}, "feedback_results": {"feedback"}, "result_actions": {"action"},
	"stalled_actions": {},
}

type Sample struct {
	Name   string
	Labels []string
	Value  float64
}
type productCollector struct {
	mu           sync.RWMutex
	samples      []Sample
	descriptions map[string]*prometheus.Desc
}

func newCollector() *productCollector {
	c := &productCollector{descriptions: map[string]*prometheus.Desc{}}
	for name, labels := range specs {
		c.descriptions[name] = prometheus.NewDesc("ra_product_"+name, "Product aggregate; UTC cohorts; user identifiers are never labels.", labels, nil)
	}
	return c
}
func (c *productCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptions {
		ch <- desc
	}
}
func (c *productCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, s := range c.samples {
		ch <- prometheus.MustNewConstMetric(c.descriptions[s.Name], prometheus.GaugeValue, s.Value, s.Labels...)
	}
}

// StartAggregator computes into an atomic snapshot. Scrapes never execute SQL.
func StartAggregator(pool *pgxpool.Pool) {
	c := newCollector()
	prometheus.MustRegister(c)
	go func() {
		for {
			delay := 15 * time.Minute
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			samples, err := Aggregate(ctx, pool, time.Now().UTC())
			cancel()
			if err != nil {
				delay = 30 * time.Second
				aggregationErrors.Inc()
				log.Printf("product aggregation: %v", err)
			} else {
				c.mu.Lock()
				c.samples = samples
				c.mu.Unlock()
			}
			time.Sleep(delay)
		}
	}()
}

// Aggregate uses one read-only repeatable snapshot for all numerators and denominators.
func Aggregate(ctx context.Context, pool *pgxpool.Pool, now time.Time) ([]Sample, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL TIME ZONE 'UTC'`); err != nil {
		return nil, err
	}
	var samples []Sample
	add := func(name string, value float64, labels ...string) {
		samples = append(samples, Sample{Name: name, Value: value, Labels: labels})
	}
	var started time.Time
	var registered float64
	if err = tx.QueryRow(ctx, `SELECT started_at,(SELECT count(*)::float8 FROM analytics_product_users) FROM analytics_settings WHERE singleton`).Scan(&started, &registered); err != nil {
		return nil, err
	}
	add("tracking_started_timestamp_seconds", float64(started.Unix()))
	add("registered_users", registered)
	for _, window := range []struct {
		name string
		days int
	}{{"1d", 1}, {"7d", 7}, {"30d", 30}} {
		var active, value float64
		err = tx.QueryRow(ctx, `SELECT
		 count(DISTINCT user_id) FILTER(WHERE event_type IN ('action_started','result_viewed','project_opened','paper_opened','paper_saved','note_created'))::float8,
		 count(DISTINCT user_id) FILTER(WHERE event_type='result_viewed')::float8
		 FROM analytics_product_events WHERE occurred_at >= $1::timestamptz-make_interval(days=>$2) AND occurred_at<=$1`, now, window.days).Scan(&active, &value)
		if err != nil {
			return nil, fmt.Errorf("active users: %w", err)
		}
		add("active_users", active, window.name)
		add("value_users", value, window.name)
	}
	var eligible, activated float64
	var p50, p90 *float64
	err = tx.QueryRow(ctx, `WITH cohort AS (
	 SELECT id,created_at FROM analytics_product_users WHERE created_at >= $2 AND created_at >= $1::timestamptz-interval '37 days' AND created_at <= $1::timestamptz-interval '7 days'
	), first_value AS (
	 SELECT c.id,c.created_at,min(e.occurred_at) AS first_at FROM cohort c LEFT JOIN analytics_product_events e
	 ON e.user_id=c.id AND e.event_type='result_viewed' AND e.occurred_at>=c.created_at AND e.occurred_at<=c.created_at+interval '7 days' GROUP BY c.id,c.created_at
	) SELECT count(*)::float8,count(first_at)::float8,
	 percentile_cont(0.5) WITHIN GROUP(ORDER BY extract(epoch FROM first_at-created_at))::float8,
	 percentile_cont(0.9) WITHIN GROUP(ORDER BY extract(epoch FROM first_at-created_at))::float8 FROM first_value`, now, started).Scan(&eligible, &activated, &p50, &p90)
	if err != nil {
		return nil, fmt.Errorf("activation: %w", err)
	}
	add("activation_users", eligible, "eligible")
	add("activation_users", activated, "activated")
	if p50 != nil {
		add("time_to_first_value_seconds", *p50, "0.5")
	}
	if p90 != nil {
		add("time_to_first_value_seconds", *p90, "0.9")
	}
	rows, err := tx.Query(ctx, `WITH periods(name,first_day,last_day) AS(VALUES('d7',7,7),('d30',30,30),('w1',1,7),('w2',8,14),('w4',22,28)), cohorts AS(
	 SELECT p.*,u.id,u.created_at::date AS cohort_date FROM periods p JOIN analytics_product_users u ON u.created_at >= $2
	 AND u.created_at::date BETWEEN ($1::timestamptz::date-p.last_day-30) AND ($1::timestamptz::date-p.last_day-1)
	) SELECT p.name,count(c.id)::float8,count(c.id) FILTER(WHERE EXISTS(
	 SELECT 1 FROM analytics_product_events e WHERE e.user_id=c.id AND e.event_type IN ('action_started','result_viewed','project_opened','paper_opened','paper_saved','note_created')
	 AND e.occurred_at::date BETWEEN c.cohort_date+c.first_day AND c.cohort_date+c.last_day
	))::float8 FROM periods p LEFT JOIN cohorts c ON c.name=p.name GROUP BY p.name`, now, started)
	if err != nil {
		return nil, fmt.Errorf("retention: %w", err)
	}
	for rows.Next() {
		var period string
		var total, returned float64
		if err = rows.Scan(&period, &total, &returned); err != nil {
			rows.Close()
			return nil, err
		}
		add("retention_users", total, period, "eligible")
		add("retention_users", returned, period, "returned")
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `WITH active AS(
	 SELECT DISTINCT user_id FROM analytics_product_events WHERE occurred_at >= $1::timestamptz-interval '30 days' AND occurred_at <= $1
	 AND event_type IN ('action_started','result_viewed','project_opened','paper_opened','paper_saved','note_created')
	), features(feature) AS(VALUES('summary'),('translation'),('chat'),('explanation'),('search'),('deep_research')), uses AS(
	 SELECT user_id,feature,count(*)::float8 AS n FROM analytics_product_events WHERE event_type='result_viewed'
	 AND occurred_at >= $1::timestamptz-interval '30 days' AND occurred_at <= $1 GROUP BY user_id,feature
	), distribution AS(SELECT f.feature,a.user_id,coalesce(u.n,0) AS n FROM features f CROSS JOIN active a LEFT JOIN uses u ON u.feature=f.feature AND u.user_id=a.user_id)
	SELECT f.feature,count(d.user_id) FILTER(WHERE d.n>0)::float8,coalesce(sum(d.n),0)::float8,
	 percentile_cont(0.5) WITHIN GROUP(ORDER BY d.n)::float8,percentile_cont(0.9) WITHIN GROUP(ORDER BY d.n)::float8
	FROM features f LEFT JOIN distribution d ON d.feature=f.feature GROUP BY f.feature`, now)
	if err != nil {
		return nil, fmt.Errorf("feature adoption: %w", err)
	}
	for rows.Next() {
		var feature string
		var users, uses float64
		var median, p90 *float64
		if err = rows.Scan(&feature, &users, &uses, &median, &p90); err != nil {
			rows.Close()
			return nil, err
		}
		add("feature_users", users, feature)
		add("feature_uses", uses, feature)
		if median != nil {
			add("feature_usage_quantile", *median, feature, "0.5")
		}
		if p90 != nil {
			add("feature_usage_quantile", *p90, feature, "0.9")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT feature,outcome,count(*)::float8 FROM analytics_product_events WHERE event_type='action_finished' AND occurred_at>=$1::timestamptz-interval '30 days' AND occurred_at<=$1 GROUP BY feature,outcome`, now)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var feature, outcome string
		var n float64
		if err = rows.Scan(&feature, &outcome, &n); err != nil {
			rows.Close()
			return nil, err
		}
		add("action_results", n, feature, outcome)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `WITH periods(days) AS(VALUES(7),(30)) SELECT p.days::text||'d',count(a.project_id)::float8,count(a.project_id) FILTER(WHERE EXISTS(
	 SELECT 1 FROM analytics_product_events e WHERE e.project_id=a.project_id AND e.user_id=a.user_id
	 AND e.event_type IN ('project_opened','paper_opened','paper_saved','note_created','action_started','result_viewed')
	 AND e.occurred_at::date>a.created_at::date AND e.occurred_at<=a.created_at+make_interval(days=>p.days)
	))::float8 FROM periods p LEFT JOIN analytics_product_projects a ON a.created_at>=$2 AND a.created_at BETWEEN $1::timestamptz-make_interval(days=>p.days+30) AND $1::timestamptz-make_interval(days=>p.days) GROUP BY p.days`, now, started)
	if err != nil {
		return nil, fmt.Errorf("project return: %w", err)
	}
	for rows.Next() {
		var period string
		var total, returned float64
		if err = rows.Scan(&period, &total, &returned); err != nil {
			rows.Close()
			return nil, err
		}
		add("project_return_projects", total, period, "eligible")
		add("project_return_projects", returned, period, "returned")
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	err = tx.QueryRow(ctx, `SELECT count(*)::float8,count(*) FILTER(WHERE EXISTS(SELECT 1 FROM analytics_product_events e WHERE e.project_id=p.project_id AND e.user_id=p.user_id
	 AND e.event_type IN ('paper_saved','note_created','action_started') AND e.occurred_at BETWEEN p.created_at AND p.created_at+interval '7 days'))::float8
	 FROM analytics_product_projects p WHERE p.created_at>=$2 AND p.created_at BETWEEN $1::timestamptz-interval '37 days' AND $1::timestamptz-interval '7 days'`, now, started).Scan(&eligible, &activated)
	if err != nil {
		return nil, err
	}
	add("project_activation_projects", eligible, "eligible")
	add("project_activation_projects", activated, "activated")
	rows, err = tx.Query(ctx, `WITH searches AS(SELECT user_id,action_id,feature,occurred_at FROM analytics_product_events WHERE event_type='action_finished' AND outcome='success' AND feature IN ('search','deep_research')
	 AND occurred_at BETWEEN $1::timestamptz-interval '37 days' AND $1::timestamptz-interval '7 days')
	SELECT feature,count(*)::float8,
	count(*) FILTER(WHERE EXISTS(SELECT 1 FROM analytics_product_events e WHERE e.user_id=s.user_id AND e.source_action_id=s.action_id AND e.event_type='paper_opened' AND e.occurred_at BETWEEN s.occurred_at AND s.occurred_at+interval '7 days'))::float8,
	count(*) FILTER(WHERE EXISTS(SELECT 1 FROM analytics_product_events e WHERE e.user_id=s.user_id AND e.source_action_id=s.action_id AND e.event_type='paper_saved' AND e.occurred_at BETWEEN s.occurred_at AND s.occurred_at+interval '7 days'))::float8
	FROM searches s GROUP BY feature`, now)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var feature string
		var completed, opened, saved float64
		if err = rows.Scan(&feature, &completed, &opened, &saved); err != nil {
			rows.Close()
			return nil, err
		}
		add("search_funnel_actions", completed, feature, "completed")
		add("search_funnel_actions", opened, feature, "opened")
		add("search_funnel_actions", saved, feature, "saved")
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT CASE WHEN feedback=1 THEN 'helpful' ELSE 'unhelpful' END,count(*)::float8 FROM analytics_product_events WHERE event_type='feedback' AND occurred_at>=$1::timestamptz-interval '30 days' AND occurred_at<=$1 GROUP BY feedback`, now)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var feedback string
		var n float64
		if err = rows.Scan(&feedback, &n); err != nil {
			rows.Close()
			return nil, err
		}
		add("feedback_results", n, feedback)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT event_type,count(*)::float8 FROM analytics_product_events WHERE event_type IN ('result_copied','paper_saved','note_created') AND occurred_at>=$1::timestamptz-interval '30 days' AND occurred_at<=$1 GROUP BY event_type`, now)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var action string
		var n float64
		if err = rows.Scan(&action, &n); err != nil {
			rows.Close()
			return nil, err
		}
		add("result_actions", n, action)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var stalled float64
	err = tx.QueryRow(ctx, `SELECT count(*)::float8 FROM analytics_product_events s WHERE s.event_type='action_started' AND s.occurred_at BETWEEN $1::timestamptz-interval '30 days' AND $1::timestamptz-interval '30 minutes'
	 AND NOT EXISTS(SELECT 1 FROM analytics_product_events e WHERE e.user_id=s.user_id AND e.action_id=s.action_id AND e.event_type='action_finished')`, now).Scan(&stalled)
	if err != nil {
		return nil, err
	}
	add("stalled_actions", stalled)
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	add("aggregation_last_success_timestamp_seconds", float64(now.Unix()))
	return samples, nil
}
