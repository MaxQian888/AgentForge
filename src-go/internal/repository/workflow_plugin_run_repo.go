package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type WorkflowPluginRunRepository struct {
	mu   sync.RWMutex
	runs map[uuid.UUID]*model.WorkflowPluginRun
}

func NewWorkflowPluginRunRepository() *WorkflowPluginRunRepository {
	return &WorkflowPluginRunRepository{
		runs: make(map[uuid.UUID]*model.WorkflowPluginRun),
	}
}

func (r *WorkflowPluginRunRepository) Create(_ context.Context, run *model.WorkflowPluginRun) error {
	if run == nil {
		return fmt.Errorf("workflow plugin run is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[run.ID] = cloneWorkflowPluginRun(run)
	return nil
}

func (r *WorkflowPluginRunRepository) Update(_ context.Context, run *model.WorkflowPluginRun) error {
	if run == nil {
		return fmt.Errorf("workflow plugin run is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[run.ID]; !ok {
		return ErrNotFound
	}
	r.runs[run.ID] = cloneWorkflowPluginRun(run)
	return nil
}

func (r *WorkflowPluginRunRepository) GetByID(_ context.Context, id uuid.UUID) (*model.WorkflowPluginRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	run, ok := r.runs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneWorkflowPluginRun(run), nil
}

func (r *WorkflowPluginRunRepository) ListByPluginID(_ context.Context, pluginID string, limit int) ([]*model.WorkflowPluginRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runs := make([]*model.WorkflowPluginRun, 0)
	for _, run := range r.runs {
		if run.PluginID != pluginID {
			continue
		}
		runs = append(runs, cloneWorkflowPluginRun(run))
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

// WorkflowPluginRunListFilter narrows a project-scoped plugin run listing.
// All fields are optional; zero values mean "no filter on this dimension".
// Used by the unified workflow-run view service so plugin-run filtering
// happens in the repo before the cross-engine merge.
type WorkflowPluginRunListFilter struct {
	Statuses         []model.WorkflowRunStatus
	ActingEmployeeID *uuid.UUID
	TriggerID        *uuid.UUID
	TriggeredByKind  string
	StartedAfter     *time.Time
	StartedBefore    *time.Time
}

// ListByProject returns plugin runs whose ProjectID equals the argument,
// newest first (StartedAt DESC, ID as tiebreaker). Filters narrow the result
// before the limit is applied. A zero limit returns every match.
func (r *WorkflowPluginRunRepository) ListByProject(_ context.Context, projectID uuid.UUID, filter WorkflowPluginRunListFilter, limit int) ([]*model.WorkflowPluginRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	statusSet := map[model.WorkflowRunStatus]struct{}{}
	for _, s := range filter.Statuses {
		statusSet[s] = struct{}{}
	}
	runs := make([]*model.WorkflowPluginRun, 0)
	for _, run := range r.runs {
		if run.ProjectID != projectID {
			continue
		}
		if len(statusSet) > 0 {
			if _, ok := statusSet[run.Status]; !ok {
				continue
			}
		}
		if filter.ActingEmployeeID != nil {
			if run.ActingEmployeeID == nil || *run.ActingEmployeeID != *filter.ActingEmployeeID {
				continue
			}
		}
		if filter.TriggerID != nil {
			if run.TriggerID == nil || *run.TriggerID != *filter.TriggerID {
				continue
			}
		}
		if filter.TriggeredByKind != "" {
			source, _ := run.Trigger["source"].(string)
			switch filter.TriggeredByKind {
			case "trigger":
				if source != "workflow.trigger" {
					continue
				}
			case "manual":
				if source != "" && source != "manual" {
					continue
				}
			case "sub_workflow":
				if source != "sub_workflow" {
					continue
				}
			default:
				// unknown triggered-by kinds never match
				continue
			}
		}
		if filter.StartedAfter != nil && !run.StartedAt.After(*filter.StartedAfter) {
			continue
		}
		if filter.StartedBefore != nil && !run.StartedAt.Before(*filter.StartedBefore) {
			continue
		}
		runs = append(runs, cloneWorkflowPluginRun(run))
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].StartedAt.Equal(runs[j].StartedAt) {
			return runs[i].StartedAt.After(runs[j].StartedAt)
		}
		return runs[i].ID.String() > runs[j].ID.String()
	})
	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

// ListByActingEmployee returns plugin runs whose ActingEmployeeID matches the
// given identifier, newest first. The in-memory seam keeps the legacy plugin
// run aggregate in parity with the DAG workflow execution repository's
// ListExecutionsByActingEmployee so consumers need no agent_run JOIN.
func (r *WorkflowPluginRunRepository) ListByActingEmployee(_ context.Context, employeeID uuid.UUID, limit int) ([]*model.WorkflowPluginRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runs := make([]*model.WorkflowPluginRun, 0)
	for _, run := range r.runs {
		if run.ActingEmployeeID == nil || *run.ActingEmployeeID != employeeID {
			continue
		}
		runs = append(runs, cloneWorkflowPluginRun(run))
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

func cloneWorkflowPluginRun(run *model.WorkflowPluginRun) *model.WorkflowPluginRun {
	if run == nil {
		return nil
	}
	cloned := *run
	if run.ActingEmployeeID != nil {
		actingID := *run.ActingEmployeeID
		cloned.ActingEmployeeID = &actingID
	}
	if run.TriggerID != nil {
		triggerID := *run.TriggerID
		cloned.TriggerID = &triggerID
	}
	if run.Trigger != nil {
		cloned.Trigger = cloneMap(run.Trigger)
	}
	if run.CompletedAt != nil {
		completedAt := *run.CompletedAt
		cloned.CompletedAt = &completedAt
	}
	if run.Steps != nil {
		cloned.Steps = make([]model.WorkflowStepRun, len(run.Steps))
		for i, step := range run.Steps {
			cloned.Steps[i] = step
			if step.Input != nil {
				cloned.Steps[i].Input = cloneMap(step.Input)
			}
			if step.Output != nil {
				cloned.Steps[i].Output = cloneMap(step.Output)
			}
			if step.StartedAt != nil {
				startedAt := *step.StartedAt
				cloned.Steps[i].StartedAt = &startedAt
			}
			if step.CompletedAt != nil {
				completedAt := *step.CompletedAt
				cloned.Steps[i].CompletedAt = &completedAt
			}
			if step.Attempts != nil {
				cloned.Steps[i].Attempts = make([]model.WorkflowStepAttempt, len(step.Attempts))
				for j, attempt := range step.Attempts {
					cloned.Steps[i].Attempts[j] = attempt
					if attempt.Output != nil {
						cloned.Steps[i].Attempts[j].Output = cloneMap(attempt.Output)
					}
					if attempt.CompletedAt != nil {
						completedAt := *attempt.CompletedAt
						cloned.Steps[i].Attempts[j].CompletedAt = &completedAt
					}
				}
			}
		}
	}
	return &cloned
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			cloned[key] = cloneMap(typed)
		case []string:
			cloned[key] = append([]string(nil), typed...)
		case []any:
			items := make([]any, len(typed))
			for i, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					items[i] = cloneMap(nested)
					continue
				}
				items[i] = item
			}
			cloned[key] = items
		default:
			cloned[key] = value
		}
	}
	return cloned
}
