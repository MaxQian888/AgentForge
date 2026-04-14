package model

import (
	"reflect"
	"testing"
)

func TestScheduledJobRunStatusIsTerminalForCancelledState(t *testing.T) {
	if ScheduledJobRunStatus("cancel_requested").IsTerminal() {
		t.Fatal("cancel_requested should remain non-terminal until cancellation settles")
	}

	if !ScheduledJobRunStatus("cancelled").IsTerminal() {
		t.Fatal("cancelled should be treated as a terminal run state")
	}
}

func TestScheduledJobExposesOperatorProjectionFields(t *testing.T) {
	jobType := reflect.TypeOf(ScheduledJob{})
	requiredFields := []string{
		"ControlState",
		"ActiveRun",
		"SupportedActions",
		"ConfigMetadata",
		"UpcomingRuns",
	}

	for _, fieldName := range requiredFields {
		if _, ok := jobType.FieldByName(fieldName); !ok {
			t.Fatalf("ScheduledJob is missing operator projection field %q", fieldName)
		}
	}
}

func TestSchedulerStatsExposeOperatorMetricsFields(t *testing.T) {
	statsType := reflect.TypeOf(SchedulerStats{})
	requiredFields := []string{
		"PausedJobs",
		"QueueDepth",
		"SuccessfulRuns24h",
		"AverageDurationMs",
		"SuccessRate24h",
	}

	for _, fieldName := range requiredFields {
		if _, ok := statsType.FieldByName(fieldName); !ok {
			t.Fatalf("SchedulerStats is missing operator metrics field %q", fieldName)
		}
	}
}
