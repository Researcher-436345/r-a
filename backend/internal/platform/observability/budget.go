package observability

import (
	"math"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

func registerBudget(service string) {
	if service != "library" && service != "api" {
		return
	}
	value, err := strconv.ParseFloat(os.Getenv("LLM_DAILY_BUDGET_USD"), 64)
	if err != nil || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}
	g := prometheus.NewGauge(prometheus.GaugeOpts{Name: "ra_llm_daily_budget_dollars", Help: "Operator-selected reported-cost budget over a rolling 24 hours."})
	g.Set(value)
	prometheus.MustRegister(g)
}
