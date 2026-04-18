package scheduler

import (
	"fmt"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
)

type CatalogConfig struct {
	TaskProgressDetectorInterval time.Duration
	ExecutionMode                model.ScheduledJobExecutionMode
}

func BuiltInCatalog(cfg CatalogConfig) []CatalogEntry {
	executionMode := cfg.ExecutionMode
	if executionMode == "" {
		executionMode = model.ScheduledJobExecutionModeInProcess
	}

	return []CatalogEntry{
		{
			JobKey:        "task-progress-detector",
			Name:          "Task progress detector",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      scheduleFromInterval(cfg.TaskProgressDetectorInterval),
			Enabled:       true,
			ExecutionMode: executionMode,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
		{
			JobKey:        "worktree-garbage-collector",
			Name:          "Worktree garbage collector",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "0 * * * *",
			Enabled:       true,
			ExecutionMode: executionMode,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
		{
			JobKey:        "bridge-health-reconcile",
			Name:          "Bridge health reconcile",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "*/10 * * * *",
			Enabled:       true,
			ExecutionMode: executionMode,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
		{
			JobKey:        "cost-reconcile",
			Name:          "Cost reconcile",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "*/15 * * * *",
			Enabled:       true,
			ExecutionMode: executionMode,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
		{
			JobKey:        "automation-due-date-detector",
			Name:          "Automation due date detector",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "*/15 * * * *",
			Enabled:       true,
			ExecutionMode: executionMode,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
		{
			JobKey:        "scheduler-history-retention",
			Name:          "Scheduler history retention",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "0 3 * * *",
			Enabled:       false,
			ExecutionMode: executionMode,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
		{
			JobKey:        "invitation-expire-sweeper",
			Name:          "Invitation expire sweeper",
			Scope:         model.ScheduledJobScopeSystem,
			Schedule:      "*/15 * * * *",
			Enabled:       true,
			ExecutionMode: executionMode,
			OverlapPolicy: model.ScheduledJobOverlapSkip,
		},
	}
}

func scheduleFromInterval(interval time.Duration) string {
	if interval <= 0 {
		return "*/1 * * * *"
	}
	if interval%time.Hour == 0 {
		hours := int(interval / time.Hour)
		switch {
		case hours <= 1:
			return "0 * * * *"
		case hours < 24:
			return fmt.Sprintf("0 */%d * * *", hours)
		case hours%24 == 0:
			days := hours / 24
			if days <= 1 {
				return "0 0 * * *"
			}
		}
	}
	if interval%time.Minute == 0 {
		minutes := int(interval / time.Minute)
		switch {
		case minutes <= 1:
			return "*/1 * * * *"
		case minutes < 60:
			return fmt.Sprintf("*/%d * * * *", minutes)
		}
	}
	return "*/1 * * * *"
}
