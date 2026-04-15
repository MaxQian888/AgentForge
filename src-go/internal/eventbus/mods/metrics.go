package mods

import (
	"context"
	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	counter *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		counter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "agentforge",
				Subsystem: "eventbus",
				Name:      "events_observed_total",
				Help:      "Events passing observe stage, labelled by type.",
			},
			[]string{"type"},
		),
	}
}

func (m *Metrics) Collector() prometheus.Collector { return m.counter }
func (m *Metrics) Name() string                     { return "core.metrics" }
func (m *Metrics) Intercepts() []string             { return []string{"*"} }
func (m *Metrics) Priority() int                    { return 90 }
func (m *Metrics) Mode() eb.Mode                    { return eb.ModeObserve }
func (m *Metrics) Observe(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) {
	m.counter.WithLabelValues(e.Type).Inc()
}
