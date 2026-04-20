package mods

import (
	"context"
	"testing"

	eb "github.com/agentforge/server/internal/eventbus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestMetrics_CountsByType(t *testing.T) {
	m := NewMetrics()
	for i := 0; i < 3; i++ {
		m.Observe(context.Background(), eb.NewEvent("task.created", "core", "task:1"), &eb.PipelineCtx{})
	}
	m.Observe(context.Background(), eb.NewEvent("agent.started", "core", "agent:r-1"), &eb.PipelineCtx{})

	count := func(typ string) float64 {
		mets := &dto.Metric{}
		require.NoError(t, m.counter.WithLabelValues(typ).Write(mets))
		return mets.Counter.GetValue()
	}
	require.Equal(t, 3.0, count("task.created"))
	require.Equal(t, 1.0, count("agent.started"))
}
