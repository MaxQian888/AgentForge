package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"

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

func cloneWorkflowPluginRun(run *model.WorkflowPluginRun) *model.WorkflowPluginRun {
	if run == nil {
		return nil
	}
	cloned := *run
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
