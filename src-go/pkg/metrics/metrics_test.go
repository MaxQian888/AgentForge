package metrics

import (
	"bytes"
	"log"
	"slices"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRegisterRegistersAllCollectors(t *testing.T) {
	originalRegisterer := prometheus.DefaultRegisterer
	originalGatherer := prometheus.DefaultGatherer
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = originalRegisterer
		prometheus.DefaultGatherer = originalGatherer
	})

	Register()

	AgentSpawnTotal.WithLabelValues("claude_code", "openai", "success").Inc()
	AgentPoolActive.Set(1)
	TaskDecomposeTotal.WithLabelValues("success").Inc()
	ReviewTotal.WithLabelValues("service", "approve").Inc()
	CostUsdTotal.WithLabelValues("claude_code", "openai").Add(1.25)
	BridgeCallDuration.WithLabelValues("health").Observe(0.05)
	TeamRunTotal.WithLabelValues("parallel", "completed").Inc()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	names := make([]string, 0, len(families))
	for _, family := range families {
		names = append(names, family.GetName())
	}
	slices.Sort(names)

	want := []string{
		"agentforge_agent_pool_active",
		"agentforge_agent_spawn_total",
		"agentforge_bridge_call_duration_seconds",
		"agentforge_cost_usd_total",
		"agentforge_review_total",
		"agentforge_task_decompose_total",
		"agentforge_team_run_total",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("registered metric names = %v, want %v", names, want)
	}
}

func TestInitTracerLogsPlaceholderMessage(t *testing.T) {
	var buffer bytes.Buffer
	originalOutput := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&buffer)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	})

	InitTracer()

	got := buffer.String()
	want := "OTel tracing not configured\n"
	if got != want {
		t.Fatalf("InitTracer() log = %q, want %q", got, want)
	}
}
