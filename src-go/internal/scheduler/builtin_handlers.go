package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/worktree"
)

type TaskProgressEvaluator interface {
	EvaluateOpenTasks(ctx context.Context) (int, error)
}

type ProjectSlugLister interface {
	ListProjectSlugs(ctx context.Context) ([]string, error)
}

type WorktreeInventoryManager interface {
	Inventory(ctx context.Context, projectSlug string) (*worktree.Inventory, error)
	GarbageCollectAll(ctx context.Context, projectSlug string) ([]worktree.Inspection, error)
}

type BridgeHealthChecker interface {
	Health(ctx context.Context) error
}

type AutomationDueDateChecker interface {
	CheckDueDateApproaching(ctx context.Context, threshold time.Duration) error
}

type CostReconcileProjectRepository interface {
	List(ctx context.Context) ([]*model.Project, error)
}

type CostReconcileTaskRepository interface {
	List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error)
	UpdateSpent(ctx context.Context, id uuid.UUID, spentUsd float64, status string) error
}

type CostReconcileTeamRepository interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentTeam, error)
	UpdateSpent(ctx context.Context, id uuid.UUID, spent float64) error
}

type CostReconcileRunRepository interface {
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error)
	ListByTeam(ctx context.Context, teamID uuid.UUID) ([]*model.AgentRun, error)
}

type FileProjectSource struct {
	RepoBasePath string
}

func (s FileProjectSource) ListProjectSlugs(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.RepoBasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	slugs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slugs = append(slugs, entry.Name())
	}
	sort.Strings(slugs)
	return slugs, nil
}

func NewTaskProgressDetectorHandler(progress TaskProgressEvaluator) Handler {
	return func(ctx context.Context, _ *model.ScheduledJob, _ *model.ScheduledJobRun) (*RunResult, error) {
		if progress == nil {
			return nil, fmt.Errorf("task progress evaluator is required")
		}

		changed, err := progress.EvaluateOpenTasks(ctx)
		if err != nil {
			return nil, err
		}

		metrics, _ := json.Marshal(map[string]int{
			"changedTasks": changed,
		})
		return &RunResult{
			Summary: fmt.Sprintf("evaluated %d open tasks", changed),
			Metrics: string(metrics),
		}, nil
	}
}

func NewWorktreeGarbageCollectorHandler(projects ProjectSlugLister, manager WorktreeInventoryManager) Handler {
	return func(ctx context.Context, _ *model.ScheduledJob, _ *model.ScheduledJobRun) (*RunResult, error) {
		if projects == nil {
			return nil, fmt.Errorf("worktree project source is required")
		}
		if manager == nil {
			return nil, fmt.Errorf("worktree inventory manager is required")
		}

		projectSlugs, err := projects.ListProjectSlugs(ctx)
		if err != nil {
			return nil, err
		}

		metrics := map[string]int{
			"projects":   len(projectSlugs),
			"inspected":  0,
			"repaired":   0,
			"unresolved": 0,
		}
		for _, projectSlug := range projectSlugs {
			before, err := manager.Inventory(ctx, projectSlug)
			if err != nil {
				return nil, err
			}
			metrics["inspected"] += before.Total

			cleaned, err := manager.GarbageCollectAll(ctx, projectSlug)
			if err != nil {
				return nil, err
			}
			metrics["repaired"] += len(cleaned)

			after, err := manager.Inventory(ctx, projectSlug)
			if err != nil {
				return nil, err
			}
			metrics["unresolved"] += after.Stale
		}

		payload, _ := json.Marshal(metrics)
		return &RunResult{
			Summary: fmt.Sprintf(
				"inspected %d managed worktrees across %d projects, repaired %d stale worktrees, %d unresolved",
				metrics["inspected"],
				metrics["projects"],
				metrics["repaired"],
				metrics["unresolved"],
			),
			Metrics: string(payload),
		}, nil
	}
}

func NewBridgeHealthReconcileHandler(health BridgeHealthChecker) Handler {
	return func(ctx context.Context, _ *model.ScheduledJob, _ *model.ScheduledJobRun) (*RunResult, error) {
		if health == nil {
			return nil, fmt.Errorf("bridge health checker is required")
		}
		if err := health.Health(ctx); err != nil {
			return nil, err
		}

		payload, _ := json.Marshal(map[string]bool{"healthy": true})
		return &RunResult{
			Summary: "bridge health check passed",
			Metrics: string(payload),
		}, nil
	}
}

func NewAutomationDueDateDetectorHandler(checker AutomationDueDateChecker, threshold time.Duration) Handler {
	return func(ctx context.Context, _ *model.ScheduledJob, _ *model.ScheduledJobRun) (*RunResult, error) {
		if checker == nil {
			return nil, fmt.Errorf("automation due date checker is required")
		}
		if threshold <= 0 {
			threshold = 24 * time.Hour
		}
		if err := checker.CheckDueDateApproaching(ctx, threshold); err != nil {
			return nil, err
		}
		payload, _ := json.Marshal(map[string]any{"thresholdHours": int(threshold.Hours())})
		return &RunResult{
			Summary: fmt.Sprintf("evaluated due-date automations within %d hours", int(threshold.Hours())),
			Metrics: string(payload),
		}, nil
	}
}

func NewCostReconcileHandler(
	projects CostReconcileProjectRepository,
	tasks CostReconcileTaskRepository,
	teams CostReconcileTeamRepository,
	runs CostReconcileRunRepository,
) Handler {
	return func(ctx context.Context, _ *model.ScheduledJob, _ *model.ScheduledJobRun) (*RunResult, error) {
		if projects == nil || tasks == nil || teams == nil || runs == nil {
			return nil, fmt.Errorf("cost reconcile dependencies are required")
		}

		projectList, err := projects.List(ctx)
		if err != nil {
			return nil, err
		}

		reconciledTasks := 0
		reconciledTeams := 0
		for _, project := range projectList {
			projectTasks, _, err := tasks.List(ctx, project.ID, model.TaskListQuery{})
			if err != nil {
				return nil, err
			}
			for _, task := range projectTasks {
				taskRuns, err := runs.GetByTask(ctx, task.ID)
				if err != nil {
					return nil, err
				}
				totalSpent := sumAgentRunCost(taskRuns)
				status := ""
				if task.BudgetUsd > 0 && totalSpent >= task.BudgetUsd {
					status = model.TaskStatusBudgetExceeded
				}
				if err := tasks.UpdateSpent(ctx, task.ID, totalSpent, status); err != nil {
					return nil, err
				}
				reconciledTasks++
			}

			projectTeams, err := teams.ListByProject(ctx, project.ID)
			if err != nil {
				return nil, err
			}
			for _, team := range projectTeams {
				teamRuns, err := runs.ListByTeam(ctx, team.ID)
				if err != nil {
					return nil, err
				}
				if err := teams.UpdateSpent(ctx, team.ID, sumAgentRunCost(teamRuns)); err != nil {
					return nil, err
				}
				reconciledTeams++
			}
		}

		payload, _ := json.Marshal(map[string]int{
			"projects": len(projectList),
			"tasks":    reconciledTasks,
			"teams":    reconciledTeams,
		})
		return &RunResult{
			Summary: fmt.Sprintf("reconciled %d tasks and %d teams across %d projects", reconciledTasks, reconciledTeams, len(projectList)),
			Metrics: string(payload),
		}, nil
	}
}

func sumAgentRunCost(runs []*model.AgentRun) float64 {
	total := 0.0
	for _, run := range runs {
		total += run.CostUsd
	}
	return total
}

func CanonicalProjectSlugs(repoBasePath string) (ProjectSlugLister, error) {
	if repoBasePath == "" {
		return nil, fmt.Errorf("repo base path is required")
	}
	return FileProjectSource{RepoBasePath: filepath.Clean(repoBasePath)}, nil
}
