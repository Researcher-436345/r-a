package queue

import (
	"context"
	"errors"
	"time"

	"github.com/centraluniversity/researcher/internal/platform/observability"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func Instrument(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		start := time.Now()
		err := next.ProcessTask(ctx, t)
		kind := t.Type()
		switch kind {
		case ProcessRemotePDF, ProcessArxivPDF, FinalizeUploadedPDF, ProcessPaperParse:
		default:
			kind = "other"
		}
		outcome := "success"
		if err != nil {
			outcome = "failed"
		}
		if ctx.Err() != nil {
			outcome = "cancelled"
		}
		observability.JobResults.WithLabelValues(kind, outcome).Inc()
		observability.JobDuration.WithLabelValues(kind).Observe(time.Since(start).Seconds())
		return err
	})
}

func StartMetrics(redisURL string) {
	options, err := redisOptions(redisURL)
	if err != nil {
		return
	}
	inspector := asynq.NewInspector(options)
	depth := promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "ra_queue_tasks", Help: "Tasks in the application default queue."}, []string{"state"})
	age := promauto.NewGauge(prometheus.GaugeOpts{Name: "ra_queue_oldest_pending_age_seconds", Help: "Age of the oldest pending task."})
	updated := promauto.NewGauge(prometheus.GaugeOpts{Name: "ra_queue_last_success_timestamp_seconds", Help: "Last successful queue inspection."})
	errorsTotal := promauto.NewCounter(prometheus.CounterOpts{Name: "ra_queue_inspection_errors_total", Help: "Failed queue inspections."})
	go func() {
		for {
			info, err := inspector.GetQueueInfo("default")
			if errors.Is(err, asynq.ErrQueueNotFound) {
				info = &asynq.QueueInfo{}
				err = nil
			}
			if err != nil {
				errorsTotal.Inc()
			} else {
				for state, n := range map[string]int{"pending": info.Pending, "active": info.Active, "retry": info.Retry, "scheduled": info.Scheduled, "archived": info.Archived} {
					depth.WithLabelValues(state).Set(float64(n))
				}
				age.Set(info.Latency.Seconds())
				updated.Set(float64(time.Now().Unix()))
			}
			time.Sleep(15 * time.Second)
		}
	}()
}
