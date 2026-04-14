package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	log "github.com/sirupsen/logrus"
)

// TeamWorkflowAdapter translates team operations into workflow operations.
// It maps team strategy names to system workflow templates and delegates
// execution to the DAGWorkflowService via the template service.
type TeamWorkflowAdapter struct {
	templateSvc *WorkflowTemplateService
}

func NewTeamWorkflowAdapter(templateSvc *WorkflowTemplateService) *TeamWorkflowAdapter {
	return &TeamWorkflowAdapter{templateSvc: templateSvc}
}

// strategyToTemplateName maps team strategy names to system template names.
func strategyToTemplateName(strategy string) string {
	switch strategy {
	case "plan-code-review":
		return TemplatePlanCodeReview
	case "pipeline":
		return TemplatePipeline
	case "swarm":
		return TemplateSwarm
	case "wave-based":
		return TemplatePlanCodeReview // Fall back to plan-code-review
	default:
		return TemplatePlanCodeReview
	}
}

// StartTeamAsWorkflow creates a workflow execution from the team's strategy template.
// Returns the workflow execution ID to be stored on the AgentTeam.
func (a *TeamWorkflowAdapter) StartTeamAsWorkflow(ctx context.Context, projectID uuid.UUID, taskID uuid.UUID, strategy string, variables map[string]any) (*uuid.UUID, error) {
	name := strategyToTemplateName(strategy)

	templates, err := a.templateSvc.repo.ListTemplatesByName(ctx, name)
	if err != nil || len(templates) == 0 {
		return nil, fmt.Errorf("no system template found for strategy %q (template: %s)", strategy, name)
	}

	tmplID := templates[0].ID
	exec, err := a.templateSvc.CreateFromTemplate(ctx, tmplID, projectID, &taskID, variables)
	if err != nil {
		return nil, fmt.Errorf("start workflow from template: %w", err)
	}

	log.WithFields(log.Fields{
		"strategy":    strategy,
		"template":    name,
		"executionId": exec.ID.String(),
		"taskId":      taskID.String(),
	}).Info("team: started as workflow execution")

	return &exec.ID, nil
}

// SyncTeamStatus maps a workflow execution status to the corresponding team status.
func SyncTeamStatus(exec *model.WorkflowExecution) string {
	switch exec.Status {
	case model.WorkflowExecStatusPending:
		return model.TeamStatusPending
	case model.WorkflowExecStatusRunning:
		return model.TeamStatusExecuting
	case model.WorkflowExecStatusPaused:
		return model.TeamStatusReviewing
	case model.WorkflowExecStatusCompleted:
		return model.TeamStatusCompleted
	case model.WorkflowExecStatusFailed:
		return model.TeamStatusFailed
	case model.WorkflowExecStatusCancelled:
		return model.TeamStatusCancelled
	default:
		return model.TeamStatusPending
	}
}
