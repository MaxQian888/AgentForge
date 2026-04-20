package model

import (
	"time"

	"github.com/google/uuid"
)

type ReviewFinding struct {
	ID          string   `json:"id,omitempty"`
	Category    string   `json:"category"`
	Subcategory string   `json:"subcategory,omitempty"`
	Severity    string   `json:"severity"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Message     string   `json:"message"`
	Suggestion  string   `json:"suggestion,omitempty"`
	CWE         string   `json:"cwe,omitempty"`
	Sources     []string `json:"sources,omitempty"`
	Dismissed   bool     `json:"dismissed,omitempty"`
	// VCS outbound fields (Spec 2B §6.3).
	SuggestedPatch  string     `json:"suggestedPatch,omitempty"`
	Decision        string     `json:"decision,omitempty"`
	DecidedAt       *time.Time `json:"decidedAt,omitempty"`
	DecidedBy       *uuid.UUID `json:"decidedBy,omitempty"`
	InlineCommentID string     `json:"inlineCommentId,omitempty"`
	ActiveFixRunID  *uuid.UUID `json:"activeFixRunId,omitempty"`
}

type ReviewExecutionKind string

const (
	ReviewExecutionKindBuiltinDimension ReviewExecutionKind = "builtin_dimension"
	ReviewExecutionKindPlugin           ReviewExecutionKind = "review_plugin"
)

type ReviewExecutionStatus string

const (
	ReviewExecutionStatusCompleted ReviewExecutionStatus = "completed"
	ReviewExecutionStatusFailed    ReviewExecutionStatus = "failed"
)

type ReviewExecutionResult struct {
	ID          string                `json:"id"`
	Kind        ReviewExecutionKind   `json:"kind"`
	Status      ReviewExecutionStatus `json:"status"`
	DisplayName string                `json:"displayName,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Error       string                `json:"error,omitempty"`
}

type ReviewDecision struct {
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Comment   string    `json:"comment"`
	Timestamp time.Time `json:"timestamp"`
}

type ReviewExecutionMetadata struct {
	TriggerEvent string                  `json:"triggerEvent,omitempty"`
	ProjectID    string                  `json:"projectId,omitempty"`
	ChangedFiles []string                `json:"changedFiles,omitempty"`
	Dimensions   []string                `json:"dimensions,omitempty"`
	Results      []ReviewExecutionResult `json:"results,omitempty"`
	Decisions    []ReviewDecision        `json:"decisions,omitempty"`
}

type Review struct {
	ID                uuid.UUID                `db:"id"`
	TaskID            uuid.UUID                `db:"task_id"`
	PRURL             string                   `db:"pr_url"`
	PRNumber          int                      `db:"pr_number"`
	Layer             int                      `db:"layer"`
	Status            string                   `db:"status"`
	RiskLevel         string                   `db:"risk_level"`
	Findings          []ReviewFinding          `db:"findings"`
	ExecutionMetadata *ReviewExecutionMetadata `db:"execution_metadata"`
	Summary           string                   `db:"summary"`
	Recommendation    string                   `db:"recommendation"`
	CostUSD           float64                  `db:"cost_usd"`
	// VCS integration fields (Spec 2B §6.2).
	IntegrationID      *uuid.UUID `db:"integration_id" json:"integrationId,omitempty"`
	HeadSHA            string     `db:"head_sha" json:"headSha,omitempty"`
	BaseSHA            string     `db:"base_sha" json:"baseSha,omitempty"`
	LastReviewedSHA    string     `db:"last_reviewed_sha" json:"lastReviewedSha,omitempty"`
	SummaryCommentID   string     `db:"summary_comment_id" json:"summaryCommentId,omitempty"`
	AutomationDecision string     `db:"automation_decision" json:"automationDecision"`
	// ParentReviewID links an incremental (diff-of-diff) review to the
	// review that anchored its base SHA. Nil on initial reviews.
	ParentReviewID *uuid.UUID `db:"parent_review_id" json:"parentReviewId,omitempty"`
	// ExecutionID links this review to a workflow_executions row when the
	// review was launched through the system:code-review template path.
	// Nil on legacy reviews created before the workflow-backed refactor.
	ExecutionID *uuid.UUID `db:"execution_id" json:"executionId,omitempty"`
	// ProjectID is populated when the review is triggered by a VCS webhook
	// (derived from the integration's project_id) or from the task's
	// project_id. It's NOT persisted on the reviews table — it's derived.
	ProjectID uuid.UUID  `db:"-" json:"projectId,omitempty"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

const (
	ReviewLayerCI    = 1
	ReviewLayerQuick = 1
	ReviewLayerDeep  = 2
	ReviewLayerHuman = 3

	ReviewStatusPending      = "pending"
	ReviewStatusInProgress   = "in_progress"
	ReviewStatusCompleted    = "completed"
	ReviewStatusFailed       = "failed"
	ReviewStatusPendingHuman = "pending_human"

	ReviewRiskLevelCritical = "critical"
	ReviewRiskLevelHigh     = "high"
	ReviewRiskLevelMedium   = "medium"
	ReviewRiskLevelLow      = "low"

	ReviewRecommendationApprove        = "approve"
	ReviewRecommendationRequestChanges = "request_changes"
	ReviewRecommendationReject         = "reject"

	ReviewTriggerAgent  = "agent"
	ReviewTriggerLayer1 = "layer1"
	ReviewTriggerManual = "manual"
)

type ReviewDTO struct {
	ID                string                   `json:"id"`
	TaskID            string                   `json:"taskId"`
	PRURL             string                   `json:"prUrl"`
	PRNumber          int                      `json:"prNumber"`
	Layer             int                      `json:"layer"`
	Status            string                   `json:"status"`
	RiskLevel         string                   `json:"riskLevel"`
	Findings          []ReviewFinding          `json:"findings"`
	ExecutionMetadata *ReviewExecutionMetadata `json:"executionMetadata,omitempty"`
	Summary           string                   `json:"summary"`
	Recommendation    string                   `json:"recommendation"`
	CostUSD           float64                  `json:"costUsd"`
	ParentReviewID    string                   `json:"parentReviewId,omitempty"`
	CreatedAt         string                   `json:"createdAt"`
	UpdatedAt         string                   `json:"updatedAt"`
}

type TriggerReviewRequest struct {
	TaskID       string   `json:"taskId"`
	ProjectID    string   `json:"projectId,omitempty"`
	PRURL        string   `json:"prUrl" validate:"required"`
	PRNumber     int      `json:"prNumber"`
	Trigger      string   `json:"trigger" validate:"required,oneof=agent layer1 manual vcs_webhook"`
	Event        string   `json:"event"`
	Dimensions   []string `json:"dimensions"`
	ChangedFiles []string `json:"changedFiles"`
	Diff         string   `json:"diff"`
	// ActingEmployeeID is the UUID of the Digital Employee this review should
	// be attributed to (empty when unattributed). Set by the legacy workflow
	// step router when its `review` step declares an `employee_id` or inherits
	// the run-level acting_employee_id (change bridge-employee-attribution-legacy).
	ActingEmployeeID string `json:"actingEmployeeId,omitempty"`
	// VCS webhook trigger fields (Spec 2B §5).
	IntegrationID string         `json:"integrationId,omitempty"`
	HeadSHA       string         `json:"headSha,omitempty"`
	BaseSHA       string         `json:"baseSha,omitempty"`
	ReplyTarget   map[string]any `json:"replyTarget,omitempty"`
}

type CIReviewRequest struct {
	TaskID          string          `json:"taskId,omitempty"`
	PRURL           string          `json:"prUrl" validate:"required"`
	CISystem        string          `json:"ciSystem"`
	Status          string          `json:"status"`
	Findings        []ReviewFinding `json:"findings"`
	NeedsDeepReview *bool           `json:"needs_deep_review,omitempty"`
	Reason          string          `json:"reason,omitempty"`
	Confidence      string          `json:"confidence,omitempty"`
}

type MarkFalsePositiveRequest struct {
	FindingIDs []string `json:"findingIds" validate:"required,min=1,dive,required"`
	Reason     string   `json:"reason" validate:"required"`
}

type ReviewExecutionPlugin struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Entrypoint   string           `json:"entrypoint,omitempty"`
	SourceType   PluginSourceType `json:"sourceType,omitempty"`
	Transport    string           `json:"transport,omitempty"`
	Command      string           `json:"command,omitempty"`
	Args         []string         `json:"args,omitempty"`
	URL          string           `json:"url,omitempty"`
	Events       []string         `json:"events,omitempty"`
	FilePatterns []string         `json:"filePatterns,omitempty"`
	OutputFormat string           `json:"outputFormat,omitempty"`
}

type ReviewExecutionPlan struct {
	TriggerEvent string                  `json:"triggerEvent"`
	ChangedFiles []string                `json:"changedFiles,omitempty"`
	Dimensions   []string                `json:"dimensions,omitempty"`
	Plugins      []ReviewExecutionPlugin `json:"plugins,omitempty"`
}

type CompleteReviewRequest struct {
	RiskLevel         string                   `json:"riskLevel" validate:"required,oneof=critical high medium low"`
	Findings          []ReviewFinding          `json:"findings"`
	ExecutionMetadata *ReviewExecutionMetadata `json:"executionMetadata,omitempty"`
	Summary           string                   `json:"summary"`
	Recommendation    string                   `json:"recommendation" validate:"required,oneof=approve request_changes reject"`
	CostUSD           float64                  `json:"costUsd"`
}

type ApproveReviewRequest struct {
	Comment string `json:"comment"`
}

type RejectReviewRequest struct {
	Comment string `json:"comment"`
	Reason  string `json:"reason" validate:"required"`
}

type RequestChangesReviewRequest struct {
	Comment string `json:"comment"`
}

func (r *Review) ToDTO() ReviewDTO {
	taskID := ""
	if r.TaskID != uuid.Nil {
		taskID = r.TaskID.String()
	}
	parentReviewID := ""
	if r.ParentReviewID != nil {
		parentReviewID = r.ParentReviewID.String()
	}
	return ReviewDTO{
		ID:                r.ID.String(),
		TaskID:            taskID,
		PRURL:             r.PRURL,
		PRNumber:          r.PRNumber,
		Layer:             r.Layer,
		Status:            r.Status,
		RiskLevel:         r.RiskLevel,
		Findings:          r.Findings,
		ExecutionMetadata: CloneReviewExecutionMetadata(r.ExecutionMetadata),
		Summary:           r.Summary,
		Recommendation:    r.Recommendation,
		CostUSD:           r.CostUSD,
		ParentReviewID:    parentReviewID,
		CreatedAt:         r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         r.UpdatedAt.Format(time.RFC3339),
	}
}

func CloneReviewExecutionMetadata(metadata *ReviewExecutionMetadata) *ReviewExecutionMetadata {
	if metadata == nil {
		return nil
	}
	cloned := *metadata
	cloned.ProjectID = metadata.ProjectID
	if metadata.ChangedFiles != nil {
		cloned.ChangedFiles = append([]string(nil), metadata.ChangedFiles...)
	}
	if metadata.Dimensions != nil {
		cloned.Dimensions = append([]string(nil), metadata.Dimensions...)
	}
	if metadata.Results != nil {
		cloned.Results = append([]ReviewExecutionResult(nil), metadata.Results...)
	}
	if metadata.Decisions != nil {
		cloned.Decisions = append([]ReviewDecision(nil), metadata.Decisions...)
	}
	return &cloned
}
