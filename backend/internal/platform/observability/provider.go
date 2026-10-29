package observability

import (
	"context"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var providerResponses = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_llm_http_responses_total", Help: "Provider HTTP responses, excluding response bodies."}, []string{"model", "feature", "status"})
var summarySources = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_summary_source_results_total", Help: "Successful summary delivery by available input source."}, []string{"source", "cache"})

func ProviderResponse(ctx context.Context, model string, status int) {
	if status < 100 || status > 599 {
		status = 0
	}
	providerResponses.WithLabelValues(ModelLabel(model), Feature(ctx), strconv.Itoa(status)).Inc()
}
func SummarySource(ctx context.Context, hasFullText bool) {
	if op, ok := ctx.Value(operationKey{}).(*Operation); ok {
		op.InputSource = "abstract_only"
		if hasFullText {
			op.InputSource = "full_text"
		}
	}
}
