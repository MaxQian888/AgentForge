package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// AgentSpawnTotal counts agent spawn attempts by runtime, provider, and status.
	AgentSpawnTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "agentforge",
			Name:      "agent_spawn_total",
			Help:      "Total number of agent spawn attempts.",
		},
		[]string{"runtime", "provider", "status"},
	)

	// AgentPoolActive tracks the current number of active agents in the pool.
	AgentPoolActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "agentforge",
			Name:      "agent_pool_active",
			Help:      "Current number of active agents.",
		},
	)

	// TaskDecomposeTotal counts task decomposition attempts by status.
	TaskDecomposeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "agentforge",
			Name:      "task_decompose_total",
			Help:      "Total number of task decomposition attempts.",
		},
		[]string{"status"},
	)

	// ReviewTotal counts review executions by layer and recommendation.
	ReviewTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "agentforge",
			Name:      "review_total",
			Help:      "Total number of review executions.",
		},
		[]string{"layer", "recommendation"},
	)

	// CostUsdTotal tracks cumulative cost in USD by runtime and provider.
	CostUsdTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "agentforge",
			Name:      "cost_usd_total",
			Help:      "Cumulative cost in USD.",
		},
		[]string{"runtime", "provider"},
	)

	// BridgeCallDuration observes bridge RPC call durations by method.
	BridgeCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "agentforge",
			Name:      "bridge_call_duration_seconds",
			Help:      "Duration of bridge RPC calls in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	// TeamRunTotal counts team run executions by strategy and status.
	TeamRunTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "agentforge",
			Name:      "team_run_total",
			Help:      "Total number of team run executions.",
		},
		[]string{"strategy", "status"},
	)
)

// Register registers all AgentForge Prometheus metrics with the default registry.
func Register() {
	prometheus.MustRegister(
		AgentSpawnTotal,
		AgentPoolActive,
		TaskDecomposeTotal,
		ReviewTotal,
		CostUsdTotal,
		BridgeCallDuration,
		TeamRunTotal,
	)
}
