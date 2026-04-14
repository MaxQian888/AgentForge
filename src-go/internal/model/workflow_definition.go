package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WorkflowDefinition stores a reusable workflow DAG or template.
type WorkflowDefinition struct {
	ID           uuid.UUID       `db:"id" json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProjectID    uuid.UUID       `db:"project_id" json:"projectId"`
	Name         string          `db:"name" json:"name"`
	Description  string          `db:"description" json:"description"`
	Status       string          `db:"status" json:"status"` // draft, active, archived, template
	Category     string          `db:"category" json:"category"` // system, user, marketplace
	Nodes        json.RawMessage `db:"nodes" json:"nodes" gorm:"type:jsonb"`
	Edges        json.RawMessage `db:"edges" json:"edges" gorm:"type:jsonb"`
	TemplateVars json.RawMessage `db:"template_vars" json:"templateVars,omitempty" gorm:"type:jsonb"`
	Version      int             `db:"version" json:"version"`
	SourceID     *uuid.UUID      `db:"source_id" json:"sourceId,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updatedAt"`
}

// WorkflowNode represents a single node in the workflow DAG.
type WorkflowNode struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"` // trigger, condition, agent_dispatch, notification, status_transition, gate, parallel_split, parallel_join
	Label    string          `json:"label"`
	Position WorkflowPos     `json:"position"`
	Config   json.RawMessage `json:"config,omitempty"`
}

// WorkflowPos represents position coordinates for a node.
type WorkflowPos struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// WorkflowEdge represents a directed edge between two nodes.
type WorkflowEdge struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	Condition string `json:"condition,omitempty"` // expression evaluated at runtime
	Label     string `json:"label,omitempty"`
}

// WorkflowExecution tracks a running workflow instance.
type WorkflowExecution struct {
	ID           uuid.UUID       `db:"id" json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	WorkflowID   uuid.UUID       `db:"workflow_id" json:"workflowId"`
	ProjectID    uuid.UUID       `db:"project_id" json:"projectId"`
	TaskID       *uuid.UUID      `db:"task_id" json:"taskId,omitempty"`
	Status       string          `db:"status" json:"status"` // pending, running, completed, failed, cancelled, paused
	CurrentNodes json.RawMessage `db:"current_nodes" json:"currentNodes" gorm:"type:jsonb"` // array of node IDs currently active
	Context      json.RawMessage `db:"context" json:"context,omitempty" gorm:"type:jsonb"`  // runtime state
	DataStore    json.RawMessage `db:"data_store" json:"dataStore,omitempty" gorm:"type:jsonb"` // accumulated node outputs keyed by node ID
	ErrorMessage string          `db:"error_message" json:"errorMessage,omitempty"`
	StartedAt    *time.Time      `db:"started_at" json:"startedAt,omitempty"`
	CompletedAt  *time.Time      `db:"completed_at" json:"completedAt,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updatedAt"`
}

// WorkflowNodeExecution tracks individual node execution within a workflow run.
type WorkflowNodeExecution struct {
	ID             uuid.UUID       `db:"id" json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ExecutionID    uuid.UUID       `db:"execution_id" json:"executionId"`
	NodeID         string          `db:"node_id" json:"nodeId"`
	Status         string          `db:"status" json:"status"` // pending, running, completed, failed, skipped, waiting
	Result         json.RawMessage `db:"result" json:"result,omitempty" gorm:"type:jsonb"`
	ErrorMessage   string          `db:"error_message" json:"errorMessage,omitempty"`
	IterationIndex int             `db:"iteration_index" json:"iterationIndex"`
	StartedAt      *time.Time      `db:"started_at" json:"startedAt,omitempty"`
	CompletedAt    *time.Time      `db:"completed_at" json:"completedAt,omitempty"`
	CreatedAt      time.Time       `db:"created_at" json:"createdAt"`
}

// Workflow definition status constants.
const (
	WorkflowDefStatusDraft    = "draft"
	WorkflowDefStatusActive   = "active"
	WorkflowDefStatusArchived = "archived"
	WorkflowDefStatusTemplate = "template"
)

// Workflow definition category constants.
const (
	WorkflowCategorySystem      = "system"
	WorkflowCategoryUser        = "user"
	WorkflowCategoryMarketplace = "marketplace"
)

// Workflow execution status constants.
const (
	WorkflowExecStatusPending   = "pending"
	WorkflowExecStatusRunning   = "running"
	WorkflowExecStatusCompleted = "completed"
	WorkflowExecStatusFailed    = "failed"
	WorkflowExecStatusCancelled = "cancelled"
	WorkflowExecStatusPaused    = "paused"
)

// Workflow node type constants.
const (
	NodeTypeTrigger          = "trigger"
	NodeTypeCondition        = "condition"
	NodeTypeAgentDispatch    = "agent_dispatch"
	NodeTypeNotification     = "notification"
	NodeTypeStatusTransition = "status_transition"
	NodeTypeGate             = "gate"
	NodeTypeParallelSplit    = "parallel_split"
	NodeTypeParallelJoin     = "parallel_join"
	NodeTypeLLMAgent         = "llm_agent"
	NodeTypeFunction         = "function"
	NodeTypeHumanReview      = "human_review"
	NodeTypeWaitEvent        = "wait_event"
	NodeTypeLoop             = "loop"
	NodeTypeSubWorkflow      = "sub_workflow"
)

// Workflow node execution status constants.
const (
	NodeExecPending   = "pending"
	NodeExecRunning   = "running"
	NodeExecCompleted = "completed"
	NodeExecFailed    = "failed"
	NodeExecSkipped   = "skipped"
	NodeExecWaiting   = "waiting"
)

// WorkflowPendingReview tracks a human review request within a workflow execution.
type WorkflowPendingReview struct {
	ID          uuid.UUID       `db:"id" json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ExecutionID uuid.UUID       `db:"execution_id" json:"executionId"`
	NodeID      string          `db:"node_id" json:"nodeId"`
	ProjectID   uuid.UUID       `db:"project_id" json:"projectId"`
	ReviewerID  *uuid.UUID      `db:"reviewer_id" json:"reviewerId,omitempty"`
	Prompt      string          `db:"prompt" json:"prompt"`
	Context     json.RawMessage `db:"context" json:"context,omitempty" gorm:"type:jsonb"`
	Decision    string          `db:"decision" json:"decision"` // pending, approved, rejected
	Comment     string          `db:"comment" json:"comment"`
	CreatedAt   time.Time       `db:"created_at" json:"createdAt"`
	ResolvedAt  *time.Time      `db:"resolved_at" json:"resolvedAt,omitempty"`
}

// Review decision constants.
const (
	ReviewDecisionPending  = "pending"
	ReviewDecisionApproved = "approved"
	ReviewDecisionRejected = "rejected"
)

// WorkflowPendingReviewDTO is the API representation.
type WorkflowPendingReviewDTO struct {
	ID          string          `json:"id"`
	ExecutionID string          `json:"executionId"`
	NodeID      string          `json:"nodeId"`
	ProjectID   string          `json:"projectId"`
	ReviewerID  *string         `json:"reviewerId,omitempty"`
	Prompt      string          `json:"prompt"`
	Context     json.RawMessage `json:"context,omitempty"`
	Decision    string          `json:"decision"`
	Comment     string          `json:"comment"`
	CreatedAt   string          `json:"createdAt"`
	ResolvedAt  *string         `json:"resolvedAt,omitempty"`
}

func (r *WorkflowPendingReview) ToDTO() WorkflowPendingReviewDTO {
	dto := WorkflowPendingReviewDTO{
		ID:          r.ID.String(),
		ExecutionID: r.ExecutionID.String(),
		NodeID:      r.NodeID,
		ProjectID:   r.ProjectID.String(),
		Prompt:      r.Prompt,
		Context:     r.Context,
		Decision:    r.Decision,
		Comment:     r.Comment,
		CreatedAt:   r.CreatedAt.Format(time.RFC3339),
	}
	if r.ReviewerID != nil {
		s := r.ReviewerID.String()
		dto.ReviewerID = &s
	}
	if r.ResolvedAt != nil {
		s := r.ResolvedAt.Format(time.RFC3339)
		dto.ResolvedAt = &s
	}
	return dto
}

// WorkflowRunMapping links an agent run back to the workflow node that spawned it.
type WorkflowRunMapping struct {
	ID          uuid.UUID `db:"id" json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ExecutionID uuid.UUID `db:"execution_id" json:"executionId"`
	NodeID      string    `db:"node_id" json:"nodeId"`
	AgentRunID  uuid.UUID `db:"agent_run_id" json:"agentRunId"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

// WorkflowDefinitionDTO is the API representation.
type WorkflowDefinitionDTO struct {
	ID           string          `json:"id"`
	ProjectID    string          `json:"projectId"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Status       string          `json:"status"`
	Category     string          `json:"category"`
	Nodes        []WorkflowNode  `json:"nodes"`
	Edges        []WorkflowEdge  `json:"edges"`
	TemplateVars json.RawMessage `json:"templateVars,omitempty"`
	Version      int             `json:"version"`
	SourceID     *string         `json:"sourceId,omitempty"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
}

// WorkflowExecutionDTO is the API representation for an execution.
type WorkflowExecutionDTO struct {
	ID           string   `json:"id"`
	WorkflowID   string   `json:"workflowId"`
	ProjectID    string   `json:"projectId"`
	TaskID       *string  `json:"taskId,omitempty"`
	Status       string   `json:"status"`
	CurrentNodes []string `json:"currentNodes"`
	ErrorMessage string   `json:"errorMessage,omitempty"`
	StartedAt    *string  `json:"startedAt,omitempty"`
	CompletedAt  *string  `json:"completedAt,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

// WorkflowNodeExecutionDTO is the API representation for a node execution.
type WorkflowNodeExecutionDTO struct {
	ID           string          `json:"id"`
	ExecutionID  string          `json:"executionId"`
	NodeID       string          `json:"nodeId"`
	Status       string          `json:"status"`
	Result       json.RawMessage `json:"result,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	StartedAt    *string         `json:"startedAt,omitempty"`
	CompletedAt  *string         `json:"completedAt,omitempty"`
	CreatedAt    string          `json:"createdAt"`
}

// CreateWorkflowDefinitionRequest is the request body for creating a workflow.
type CreateWorkflowDefinitionRequest struct {
	Name        string         `json:"name" validate:"required,min=1,max=200"`
	Description string         `json:"description"`
	Nodes       []WorkflowNode `json:"nodes"`
	Edges       []WorkflowEdge `json:"edges"`
}

// UpdateWorkflowDefinitionRequest is the request body for updating a workflow.
type UpdateWorkflowDefinitionRequest struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Status      *string         `json:"status"`
	Nodes       *[]WorkflowNode `json:"nodes"`
	Edges       *[]WorkflowEdge `json:"edges"`
}

// StartWorkflowExecutionRequest is the request body for starting an execution.
type StartWorkflowExecutionRequest struct {
	TaskID *string `json:"taskId,omitempty"`
}

// ToDTO converts a WorkflowDefinition to its API representation.
func (w *WorkflowDefinition) ToDTO() WorkflowDefinitionDTO {
	dto := WorkflowDefinitionDTO{
		ID:           w.ID.String(),
		ProjectID:    w.ProjectID.String(),
		Name:         w.Name,
		Description:  w.Description,
		Status:       w.Status,
		Category:     w.Category,
		Nodes:        make([]WorkflowNode, 0),
		Edges:        make([]WorkflowEdge, 0),
		TemplateVars: w.TemplateVars,
		Version:      w.Version,
		CreatedAt:    w.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    w.UpdatedAt.Format(time.RFC3339),
	}
	if w.SourceID != nil {
		s := w.SourceID.String()
		dto.SourceID = &s
	}
	if len(w.Nodes) > 0 {
		_ = json.Unmarshal(w.Nodes, &dto.Nodes)
	}
	if len(w.Edges) > 0 {
		_ = json.Unmarshal(w.Edges, &dto.Edges)
	}
	return dto
}

// ToDTO converts a WorkflowExecution to its API representation.
func (e *WorkflowExecution) ToDTO() WorkflowExecutionDTO {
	dto := WorkflowExecutionDTO{
		ID:           e.ID.String(),
		WorkflowID:   e.WorkflowID.String(),
		ProjectID:    e.ProjectID.String(),
		Status:       e.Status,
		CurrentNodes: make([]string, 0),
		ErrorMessage: e.ErrorMessage,
		CreatedAt:    e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    e.UpdatedAt.Format(time.RFC3339),
	}
	if e.TaskID != nil {
		s := e.TaskID.String()
		dto.TaskID = &s
	}
	if len(e.CurrentNodes) > 0 {
		_ = json.Unmarshal(e.CurrentNodes, &dto.CurrentNodes)
	}
	if e.StartedAt != nil {
		s := e.StartedAt.Format(time.RFC3339)
		dto.StartedAt = &s
	}
	if e.CompletedAt != nil {
		s := e.CompletedAt.Format(time.RFC3339)
		dto.CompletedAt = &s
	}
	return dto
}

// ToDTO converts a WorkflowNodeExecution to its API representation.
func (n *WorkflowNodeExecution) ToDTO() WorkflowNodeExecutionDTO {
	dto := WorkflowNodeExecutionDTO{
		ID:           n.ID.String(),
		ExecutionID:  n.ExecutionID.String(),
		NodeID:       n.NodeID,
		Status:       n.Status,
		Result:       n.Result,
		ErrorMessage: n.ErrorMessage,
		CreatedAt:    n.CreatedAt.Format(time.RFC3339),
	}
	if n.StartedAt != nil {
		s := n.StartedAt.Format(time.RFC3339)
		dto.StartedAt = &s
	}
	if n.CompletedAt != nil {
		s := n.CompletedAt.Format(time.RFC3339)
		dto.CompletedAt = &s
	}
	return dto
}
