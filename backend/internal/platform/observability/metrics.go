package observability

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequests   = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_http_requests_total", Help: "Completed HTTP requests."}, []string{"service", "route", "method", "status"})
	HTTPDuration   = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "ra_http_request_duration_seconds", Help: "HTTP request duration.", Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 15, 60, 180, 600}}, []string{"service", "route", "method"})
	HTTPInflight   = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "ra_http_inflight_requests", Help: "In-flight HTTP requests."}, []string{"service"})
	ActionResults  = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_action_results_total", Help: "HTTP attempts by actual terminal outcome, including SSE errors."}, []string{"service", "feature", "outcome", "cache"})
	ActionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "ra_action_duration_seconds", Help: "Time to complete a research action.", Buckets: []float64{.1, .5, 1, 2.5, 5, 10, 20, 30, 60, 120, 300, 600, 1200}}, []string{"feature", "outcome"})
	FirstContent   = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "ra_action_first_content_seconds", Help: "Time to first nonempty content, excluding progress and keepalive events.", Buckets: []float64{.1, .5, 1, 2, 5, 10, 20, 30, 60, 120, 300, 600}}, []string{"feature"})
	LLMRequests    = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_llm_requests_total", Help: "Provider requests, including retries."}, []string{"model", "feature", "outcome"})
	LLMDuration    = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "ra_llm_request_duration_seconds", Help: "Provider request duration.", Buckets: []float64{.1, .5, 1, 2, 5, 10, 30, 60, 120, 300, 600}}, []string{"model", "feature"})
	LLMTokens      = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_llm_tokens_total", Help: "Reported provider token usage."}, []string{"model", "feature", "direction"})
	LLMCost        = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_llm_reported_cost_dollars_total", Help: "Cost explicitly reported by the provider; no inferred prices."}, []string{"model", "feature"})
	LLMUsage       = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_llm_usage_reports_total", Help: "Whether provider usage and cost were available."}, []string{"model", "feature", "kind"})
	CacheRequests  = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_cache_requests_total", Help: "Application cache reads."}, []string{"cache", "outcome"})
	JobResults     = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_job_attempts_total", Help: "Worker task attempts."}, []string{"type", "outcome"})
	JobDuration    = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "ra_job_duration_seconds", Help: "Worker attempt duration.", Buckets: []float64{.1, .5, 1, 5, 15, 30, 60, 120, 300, 600}}, []string{"type"})
	FeedRefresh    = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "ra_feed_refresh_last_success_timestamp_seconds", Help: "Last successfully published feed snapshot."}, []string{"sort"})
	FeedErrors     = promauto.NewCounter(prometheus.CounterOpts{Name: "ra_feed_refresh_errors_total", Help: "Feed preparation failures."})
	Pipeline       = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_paper_pipeline_results_total", Help: "Paper processing outcomes."}, []string{"stage", "outcome"})
)

// Start exposes metrics on a separate internal port; the gateway never proxies it.
func Start(service string) {
	registerBudget(service)
	address := os.Getenv("METRICS_ADDR")
	if address == "off" {
		return
	}
	if address == "" {
		address = ":9091"
	}
	info := promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "ra_service_info", Help: "Running application service."}, []string{"service"})
	info.WithLabelValues(service).Set(1)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go func() {
		server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics listener: %v", err)
		}
	}()
}

func Feature(ctx context.Context) string {
	if op, ok := ctx.Value(operationKey{}).(*Operation); ok {
		return op.Feature
	}
	return "background"
}

func ModelLabel(model string) string {
	if model == "" {
		return "unknown"
	}
	// Model identifiers come from server configuration or its allowlisted picker.
	if len(model) > 128 || strings.ContainsAny(model, "\n\r\t ") {
		return "unknown"
	}
	return model
}

func ObserveLLM(ctx context.Context, model string) func(error) {
	start := time.Now()
	return func(err error) {
		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		if ctx.Err() != nil {
			outcome = "cancelled"
		}
		model, feature := ModelLabel(model), Feature(ctx)
		LLMRequests.WithLabelValues(model, feature, outcome).Inc()
		LLMDuration.WithLabelValues(model, feature).Observe(time.Since(start).Seconds())
	}
}

// RecordUsage records only real provider values. Missing reports never become zero-cost calls.
func RecordUsage(ctx context.Context, model string, input, output, cached int64, cost *float64) {
	model, feature := ModelLabel(model), Feature(ctx)
	if input >= 0 && output >= 0 {
		LLMTokens.WithLabelValues(model, feature, "input").Add(float64(input))
		LLMTokens.WithLabelValues(model, feature, "output").Add(float64(output))
		if cached > 0 {
			LLMTokens.WithLabelValues(model, feature, "cached_input").Add(float64(cached))
		}
		LLMUsage.WithLabelValues(model, feature, "tokens").Inc()
	} else {
		LLMUsage.WithLabelValues(model, feature, "missing_tokens").Inc()
	}
	if cost != nil && *cost >= 0 {
		LLMCost.WithLabelValues(model, feature).Add(*cost)
		LLMUsage.WithLabelValues(model, feature, "cost").Inc()
	} else {
		LLMUsage.WithLabelValues(model, feature, "missing_cost").Inc()
	}
}
