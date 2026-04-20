package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
	eventbus "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

var (
	ErrReviewNotFound          = errors.New("review not found")
	ErrReviewTaskNotFound      = errors.New("review task not found")
	ErrReviewInvalidTransition = errors.New("review transition is not allowed for current state")
)

type ReviewRepository interface {
	Create(ctx context.Context, review *model.Review) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error)
	ListAll(ctx context.Context, status, riskLevel string, limit int) ([]*model.Review, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateResult(ctx context.Context, review *model.Review) error
	SetExecutionID(ctx context.Context, id uuid.UUID, executionID uuid.UUID) error
}

// ReviewWorkflowLauncher starts a workflow execution from a named template.
// In production implemented by *WorkflowTemplateService + *DAGWorkflowService
// composed behind a thin adapter. Nil disables the workflow-backed path.
type ReviewWorkflowLauncher interface {
	LaunchReviewWorkflow(ctx context.Context, projectID uuid.UUID, seed map[string]any) (executionID uuid.UUID, err error)
}

type ReviewTaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	GetByPRURL(ctx context.Context, prURL string) (*model.Task, error)
	TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
}

type ReviewProjectRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
}

type ReviewNotificationCreator interface {
	Create(ctx context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error)
}

type ReviewBridgeClient interface {
	Review(ctx context.Context, req bridgeclient.ReviewRequest) (*bridgeclient.ReviewResponse, error)
}

type ReviewService struct {
	reviews             ReviewRepository
	tasks               ReviewTaskRepository
	projects            ReviewProjectRepository
	notifications       ReviewNotificationCreator
	hub                 *ws.Hub
	bus                 eventbus.Publisher
	bridge              ReviewBridgeClient
	planner             *ReviewExecutionPlanner
	progress            *TaskProgressService
	imProgress          IMBoundProgressNotifier
	aggregation         *ReviewAggregationService
	automation          AutomationEventEvaluator
	links               entityLinkRepository
	pages               wikiPageRepository
	versions            pageVersionRepository
	workflowLauncher    ReviewWorkflowLauncher
	workflowLaunchFlag  func() bool
}

func reviewLogFields(review *model.Review, task *model.Task) log.Fields {
	fields := log.Fields{}
	if review != nil {
		fields["reviewId"] = review.ID.String()
		fields["reviewStatus"] = review.Status
		fields["riskLevel"] = review.RiskLevel
		fields["recommendation"] = review.Recommendation
		fields["prNumber"] = review.PRNumber
	}
	if task != nil {
		fields["taskId"] = task.ID.String()
		fields["projectId"] = task.ProjectID.String()
		fields["taskStatus"] = task.Status
	}
	return fields
}

func NewReviewService(
	reviews ReviewRepository,
	tasks ReviewTaskRepository,
	notifications ReviewNotificationCreator,
	hub *ws.Hub,
	bus eventbus.Publisher,
	bridge ReviewBridgeClient,
	progress ...*TaskProgressService,
) *ReviewService {
	var tracker *TaskProgressService
	if len(progress) > 0 {
		tracker = progress[0]
	}
	return &ReviewService{
		reviews:       reviews,
		tasks:         tasks,
		notifications: notifications,
		hub:           hub,
		bus:           bus,
		bridge:        bridge,
		progress:      tracker,
	}
}

func (s *ReviewService) SetIMProgressNotifier(notifier IMBoundProgressNotifier) {
	s.imProgress = notifier
}

func (s *ReviewService) WithExecutionPlanner(planner *ReviewExecutionPlanner) *ReviewService {
	s.planner = planner
	return s
}

func (s *ReviewService) WithProjectRepository(projects ReviewProjectRepository) *ReviewService {
	s.projects = projects
	return s
}

func (s *ReviewService) WithAggregationService(agg *ReviewAggregationService) *ReviewService {
	s.aggregation = agg
	return s
}

func (s *ReviewService) SetAutomationEvaluator(evaluator AutomationEventEvaluator) {
	s.automation = evaluator
}

func (s *ReviewService) WithDocWriteback(links entityLinkRepository, pages wikiPageRepository, versions pageVersionRepository) *ReviewService {
	s.links = links
	s.pages = pages
	s.versions = versions
	return s
}

// WithWorkflowLauncher attaches a workflow launcher. flag is the runtime
// gate (typically returns USE_WORKFLOW_BACKED_REVIEW == "true"). Nil flag
// is treated as always-off.
func (s *ReviewService) WithWorkflowLauncher(launcher ReviewWorkflowLauncher, flag func() bool) *ReviewService {
	s.workflowLauncher = launcher
	s.workflowLaunchFlag = flag
	return s
}

// launchWorkflowBackedReview is best-effort: any error is logged and the
// legacy review flow continues. When it succeeds, review.ExecutionID is
// stamped onto the row via SetExecutionID.
//
// The workflow runs in parallel with the existing bridge-based review.
// Both paths eventually converge on the same `reviews` row; the workflow
// path only writes execution_id.
func (s *ReviewService) launchWorkflowBackedReview(ctx context.Context, review *model.Review, projectID uuid.UUID) {
	if s.workflowLauncher == nil {
		return
	}
	if s.workflowLaunchFlag == nil || !s.workflowLaunchFlag() {
		return
	}
	if projectID == uuid.Nil {
		log.WithField("reviewId", review.ID).Debug("workflow-backed review skipped: project id unknown")
		return
	}
	seed := map[string]any{
		"review_id": review.ID.String(),
		"pr_url":    review.PRURL,
		"pr_number": review.PRNumber,
	}
	execID, err := s.workflowLauncher.LaunchReviewWorkflow(ctx, projectID, seed)
	if err != nil {
		log.WithFields(log.Fields{
			"reviewId":  review.ID.String(),
			"projectId": projectID.String(),
		}).WithError(err).Warn("workflow-backed review launch failed")
		return
	}
	if err := s.reviews.SetExecutionID(ctx, review.ID, execID); err != nil {
		log.WithFields(log.Fields{
			"reviewId":    review.ID.String(),
			"executionId": execID.String(),
		}).WithError(err).Warn("persist review.execution_id failed")
		return
	}
	review.ExecutionID = &execID
	log.WithFields(log.Fields{
		"reviewId":    review.ID.String(),
		"executionId": execID.String(),
	}).Info("workflow-backed review launched")
}

func (s *ReviewService) Trigger(ctx context.Context, req *model.TriggerReviewRequest) (*model.Review, error) {
	var (
		task      *model.Task
		err       error
		projectID uuid.UUID
	)

	if req.TaskID != "" {
		taskID, parseErr := uuid.Parse(req.TaskID)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid task id: %w", parseErr)
		}
		task, err = s.tasks.GetByID(ctx, taskID)
	} else {
		task, err = s.tasks.GetByPRURL(ctx, req.PRURL)
	}
	if err != nil {
		task = nil
	}

	if task != nil {
		projectID = task.ProjectID
	}

	detached := false
	if task == nil {
		if strings.TrimSpace(req.PRURL) == "" {
			return nil, ErrReviewTaskNotFound
		}
		detached = true
		if strings.TrimSpace(req.ProjectID) != "" {
			parsedProjectID, parseErr := uuid.Parse(strings.TrimSpace(req.ProjectID))
			if parseErr != nil {
				return nil, fmt.Errorf("invalid project id: %w", parseErr)
			}
			projectID = parsedProjectID
		}
	}

	triggerFields := log.Fields{
		"trigger":  req.Trigger,
		"prUrl":    req.PRURL,
		"prNumber": req.PRNumber,
		"detached": detached,
	}
	if task != nil {
		triggerFields["taskId"] = task.ID.String()
		triggerFields["projectId"] = task.ProjectID.String()
		triggerFields["taskStatus"] = task.Status
	} else if projectID != uuid.Nil {
		triggerFields["projectId"] = projectID.String()
	}

	executionPlan, err := s.buildExecutionPlan(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build review execution plan: %w", err)
	}
	triggerFields["dimensionCount"] = len(executionPlan.Dimensions)
	triggerFields["pluginCount"] = len(executionPlan.Plugins)
	triggerFields["changedFileCount"] = len(executionPlan.ChangedFiles)

	review := &model.Review{
		ID:        uuid.New(),
		TaskID:    uuid.Nil,
		PRURL:     req.PRURL,
		PRNumber:  req.PRNumber,
		Layer:     model.ReviewLayerDeep,
		Status:    model.ReviewStatusInProgress,
		RiskLevel: model.ReviewRiskLevelLow,
	}
	if task != nil {
		review.TaskID = task.ID
	}

	if err := s.reviews.Create(ctx, review); err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}
	triggerFields["reviewId"] = review.ID.String()
	log.WithFields(triggerFields).Info("review created")

	// Workflow-backed path (opt-in, additive — legacy bridge flow continues below).
	s.launchWorkflowBackedReview(ctx, review, projectID)

	if task != nil && task.Status == model.TaskStatusInProgress {
		if err := s.tasks.TransitionStatus(ctx, task.ID, model.TaskStatusInReview); err != nil {
			return nil, fmt.Errorf("transition task to in_review: %w", err)
		}
		task.Status = model.TaskStatusInReview
		log.WithFields(triggerFields).Info("task transitioned to in_review for review")
	}

	if projectID != uuid.Nil {
		s.broadcast(ctx, ws.EventReviewCreated, projectID.String(), review.ToDTO())
	}
	if task != nil {
		s.recordProgress(ctx, task.ID, TaskActivityInput{
			Source:         model.TaskProgressSourceReviewCreated,
			OccurredAt:     time.Now().UTC(),
			UpdateHealth:   true,
			MarkTransition: true,
		})
	}

	if s.bridge == nil {
		log.WithFields(triggerFields).Debug("review trigger completed without bridge client")
		return review, nil
	}

	taskID := ""
	taskTitle := ""
	taskDescription := ""
	if task != nil {
		taskID = task.ID.String()
		taskTitle = task.Title
		taskDescription = task.Description
	}
	result, err := s.bridge.Review(ctx, bridgeclient.ReviewRequest{
		ReviewID:      review.ID.String(),
		TaskID:        taskID,
		PRURL:         req.PRURL,
		PRNumber:      req.PRNumber,
		Title:         taskTitle,
		Description:   taskDescription,
		Diff:          req.Diff,
		Dimensions:    executionPlan.Dimensions,
		TriggerEvent:  executionPlan.TriggerEvent,
		ChangedFiles:  executionPlan.ChangedFiles,
		ReviewPlugins: reviewPluginRequestsFromPlan(executionPlan),
	})
	if err != nil {
		_ = s.reviews.UpdateStatus(ctx, review.ID, model.ReviewStatusFailed)
		log.WithFields(triggerFields).WithError(err).Warn("review bridge execution failed")
		return nil, fmt.Errorf("bridge review: %w", err)
	}
	triggerFields["findingCount"] = len(result.Findings)
	triggerFields["riskLevel"] = result.RiskLevel
	triggerFields["recommendation"] = result.Recommendation
	triggerFields["costUsd"] = result.CostUSD
	log.WithFields(triggerFields).Info("review bridge execution completed")

	executionMetadata := reviewExecutionMetadataFromBridge(executionPlan, result)
	if projectID != uuid.Nil {
		if executionMetadata == nil {
			executionMetadata = &model.ReviewExecutionMetadata{}
		}
		executionMetadata.ProjectID = projectID.String()
	}

	return s.Complete(ctx, review.ID, &model.CompleteReviewRequest{
		RiskLevel:         result.RiskLevel,
		Findings:          result.Findings,
		ExecutionMetadata: executionMetadata,
		Summary:           result.Summary,
		Recommendation:    result.Recommendation,
		CostUSD:           result.CostUSD,
	})
}

func (s *ReviewService) Complete(ctx context.Context, id uuid.UUID, req *model.CompleteReviewRequest) (*model.Review, error) {
	review, err := s.reviews.GetByID(ctx, id)
	if err != nil {
		return nil, ErrReviewNotFound
	}

	var task *model.Task
	projectID := uuid.Nil
	if review.TaskID != uuid.Nil {
		task, err = s.tasks.GetByID(ctx, review.TaskID)
		if err != nil {
			return nil, ErrReviewTaskNotFound
		}
		projectID = task.ProjectID
	}

	review.Status = model.ReviewStatusCompleted
	review.RiskLevel = req.RiskLevel
	review.Findings = req.Findings
	if req.ExecutionMetadata != nil {
		review.ExecutionMetadata = model.CloneReviewExecutionMetadata(req.ExecutionMetadata)
	}
	if projectID == uuid.Nil {
		projectID = projectIDFromMetadata(req.ExecutionMetadata, review.ExecutionMetadata)
	}
	if projectID != uuid.Nil {
		if review.ExecutionMetadata == nil {
			review.ExecutionMetadata = &model.ReviewExecutionMetadata{}
		}
		if strings.TrimSpace(review.ExecutionMetadata.ProjectID) == "" {
			review.ExecutionMetadata.ProjectID = projectID.String()
		}
	}
	review.Summary = req.Summary
	review.Recommendation = req.Recommendation
	review.CostUSD = req.CostUSD
	fields := reviewLogFields(review, task)
	fields["findingCount"] = len(req.Findings)
	fields["costUsd"] = req.CostUSD
	if projectID != uuid.Nil {
		fields["projectId"] = projectID.String()
	}

	if err := s.reviews.UpdateResult(ctx, review); err != nil {
		return nil, fmt.Errorf("update review result: %w", err)
	}
	log.WithFields(fields).Info("review result stored")

	routePendingHuman, pendingReason := s.shouldRoutePendingHuman(ctx, projectID, review)
	if routePendingHuman {
		fields["pendingHumanReason"] = pendingReason
		if err := s.RequestHumanApproval(ctx, review.ID); err != nil {
			log.WithFields(fields).WithError(err).Warn("review pending_human transition failed")
			return nil, err
		}
		latest, getErr := s.reviews.GetByID(ctx, review.ID)
		if getErr == nil && latest != nil {
			review = latest
		} else {
			review.Status = model.ReviewStatusPendingHuman
		}
		log.WithFields(fields).Info("review routed to pending_human")
		return review, nil
	}

	if task != nil {
		targetStatus := mapRecommendationToTaskStatus(req.Recommendation)
		if targetStatus != "" {
			if err := s.transitionTaskForReview(ctx, task, targetStatus); err != nil {
				log.WithFields(fields).WithError(err).Warn("review task transition failed")
				return nil, err
			}
			fields["taskStatus"] = task.Status
		}
	}

	if task != nil && task.AssigneeID != nil && s.notifications != nil {
		payload, _ := json.Marshal(review.ToDTO())
		if _, err := s.notifications.Create(
			ctx,
			*task.AssigneeID,
			model.NotificationTypeReviewCompleted,
			"Deep review completed",
			fmt.Sprintf("Layer 2 review finished for task %s with recommendation %s", task.Title, review.Recommendation),
			string(payload),
		); err != nil {
			log.WithFields(fields).WithField("assigneeId", task.AssigneeID.String()).WithError(err).Warn("review notification create failed")
		}
	}

	if task != nil {
		if docID, versionID, writebackErr := s.writeBackReviewFindings(ctx, task, review); writebackErr != nil {
			log.WithFields(fields).WithField("writebackDocumentId", docID).WithError(writebackErr).Warn("review doc writeback failed")
		} else if docID != "" {
			fields["writebackDocumentId"] = docID
			fields["writebackVersionId"] = versionID
			log.WithFields(fields).Info("review doc writeback completed")
		} else {
			log.WithFields(fields).Info("review doc writeback skipped")
		}
	}

	if projectID != uuid.Nil {
		s.broadcast(ctx, ws.EventReviewCompleted, projectID.String(), review.ToDTO())
	}
	if task != nil {
		s.recordProgress(ctx, task.ID, TaskActivityInput{
			Source:         model.TaskProgressSourceReviewComplete,
			OccurredAt:     time.Now().UTC(),
			UpdateHealth:   true,
			MarkTransition: true,
		})
		if s.imProgress != nil {
			queued, err := s.imProgress.QueueBoundProgress(ctx, IMBoundProgressRequest{
				TaskID:   task.ID.String(),
				ReviewID: review.ID.String(),
				Kind:     IMDeliveryKindTerminal,
				Content:  fmt.Sprintf("代码审查已完成。\nReview: %s\n状态: %s\n建议: %s", review.ID.String(), review.Status, review.Recommendation),
				Structured: &model.IMStructuredMessage{
					Title: "Code Review Completed",
					Body:  fmt.Sprintf("Review %s finished.", review.ID.String()),
					Fields: []model.IMStructuredField{
						{Label: "Review", Value: review.ID.String()},
						{Label: "Status", Value: review.Status},
						{Label: "Recommendation", Value: review.Recommendation},
					},
				},
				IsTerminal: true,
			})
			fields["imQueued"] = queued
			if err != nil {
				log.WithFields(fields).WithError(err).Warn("review IM progress queue failed")
			}
		}
	}
	// Trigger aggregation if service is available.
	if s.aggregation != nil && task != nil {
		if agg, aggErr := s.aggregation.Aggregate(ctx, review.TaskID); aggErr != nil {
			log.WithFields(fields).WithError(aggErr).Warn("post-complete aggregation failed")
		} else {
			log.WithFields(fields).WithField("aggregationId", agg.ID.String()).Info("post-complete aggregation succeeded")
		}
	}
	if s.automation != nil && task != nil {
		taskID := task.ID
		_ = s.automation.EvaluateRules(ctx, AutomationEvent{
			EventType: model.AutomationEventReviewCompleted,
			ProjectID: task.ProjectID,
			TaskID:    &taskID,
			Task:      task,
			Data: map[string]any{
				"review_id":      review.ID.String(),
				"recommendation": review.Recommendation,
				"risk_level":     review.RiskLevel,
				"review_status":  review.Status,
			},
		})
	}

	log.WithFields(fields).Info("review completed")
	return review, nil
}

// IngestCIResult creates a review with Layer "ci" (Layer 1) from CI pipeline findings.
func (s *ReviewService) IngestCIResult(ctx context.Context, req *model.CIReviewRequest) (*model.Review, error) {
	var (
		task *model.Task
		err  error
	)
	if strings.TrimSpace(req.TaskID) != "" {
		taskID, parseErr := uuid.Parse(strings.TrimSpace(req.TaskID))
		if parseErr != nil {
			return nil, fmt.Errorf("invalid task id: %w", parseErr)
		}
		task, err = s.tasks.GetByID(ctx, taskID)
	} else {
		task, err = s.tasks.GetByPRURL(ctx, req.PRURL)
	}
	if err != nil || task == nil {
		return nil, ErrReviewTaskNotFound
	}

	needsDeepReview := false
	if req.NeedsDeepReview != nil {
		needsDeepReview = *req.NeedsDeepReview
	}
	if strings.EqualFold(req.Status, "failure") || strings.EqualFold(req.Status, "error") {
		needsDeepReview = true
	}
	if len(req.Findings) > 0 {
		needsDeepReview = true
	}

	// Determine risk level from CI status and escalation decision.
	riskLevel := model.ReviewRiskLevelLow
	recommendation := model.ReviewRecommendationApprove
	if strings.EqualFold(req.Status, "failure") || strings.EqualFold(req.Status, "error") {
		riskLevel = model.ReviewRiskLevelHigh
		recommendation = model.ReviewRecommendationRequestChanges
	} else if needsDeepReview {
		riskLevel = model.ReviewRiskLevelMedium
		recommendation = model.ReviewRecommendationRequestChanges
	}

	summary := fmt.Sprintf("CI result from %s: status=%s, findings=%d", strings.TrimSpace(req.CISystem), strings.TrimSpace(req.Status), len(req.Findings))
	if reason := strings.TrimSpace(req.Reason); reason != "" {
		summary += fmt.Sprintf(", reason=%s", reason)
	}
	if confidence := strings.TrimSpace(req.Confidence); confidence != "" {
		summary += fmt.Sprintf(", confidence=%s", confidence)
	}
	if needsDeepReview {
		summary += ", needs_deep_review=true"
	}

	review := &model.Review{
		ID:             uuid.New(),
		TaskID:         task.ID,
		PRURL:          req.PRURL,
		Layer:          model.ReviewLayerCI,
		Status:         model.ReviewStatusCompleted,
		RiskLevel:      riskLevel,
		Findings:       req.Findings,
		Summary:        summary,
		Recommendation: recommendation,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.reviews.Create(ctx, review); err != nil {
		return nil, fmt.Errorf("create CI review: %w", err)
	}

	fields := reviewLogFields(review, task)
	fields["ciSystem"] = req.CISystem
	fields["findingCount"] = len(req.Findings)
	fields["needsDeepReview"] = needsDeepReview
	log.WithFields(fields).Info("CI review ingested")

	s.broadcast(ctx, ws.EventReviewCreated, task.ProjectID.String(), review.ToDTO())

	// Trigger aggregation if available.
	if s.aggregation != nil {
		if agg, aggErr := s.aggregation.Aggregate(ctx, task.ID); aggErr != nil {
			log.WithFields(fields).WithError(aggErr).Warn("post-CI aggregation failed")
		} else {
			log.WithFields(fields).WithField("aggregationId", agg.ID.String()).Info("post-CI aggregation succeeded")
		}
	}

	return review, nil
}

// RequestHumanApproval sets a review to pending_human status and broadcasts
// an event so IM/UI can notify the appropriate reviewer.
func (s *ReviewService) RequestHumanApproval(ctx context.Context, reviewID uuid.UUID) error {
	review, err := s.reviews.GetByID(ctx, reviewID)
	if err != nil {
		return ErrReviewNotFound
	}

	var (
		task      *model.Task
		projectID uuid.UUID
	)
	if review.TaskID != uuid.Nil {
		task, err = s.tasks.GetByID(ctx, review.TaskID)
		if err != nil {
			return ErrReviewTaskNotFound
		}
		projectID = task.ProjectID
	} else {
		projectID = projectIDFromMetadata(review.ExecutionMetadata)
	}

	if err := s.reviews.UpdateStatus(ctx, reviewID, model.ReviewStatusPendingHuman); err != nil {
		return fmt.Errorf("update review to pending_human: %w", err)
	}

	review.Status = model.ReviewStatusPendingHuman
	fields := reviewLogFields(review, task)
	log.WithFields(fields).Info("review set to pending_human")

	if projectID != uuid.Nil {
		s.broadcast(ctx, ws.EventReviewPendingHuman, projectID.String(), review.ToDTO())
	}

	if task != nil && task.AssigneeID != nil && s.notifications != nil {
		payload, _ := json.Marshal(review.ToDTO())
		if _, err := s.notifications.Create(
			ctx,
			*task.AssigneeID,
			model.NotificationTypeReviewCompleted,
			"Human review requested",
			fmt.Sprintf("Review %s for task %s requires human approval", review.ID.String(), task.Title),
			string(payload),
		); err != nil {
			log.WithFields(fields).WithError(err).Warn("human approval notification failed")
		}
	}

	return nil
}


var _ interface {
	Trigger(context.Context, *model.TriggerReviewRequest) (*model.Review, error)
	Complete(context.Context, uuid.UUID, *model.CompleteReviewRequest) (*model.Review, error)
	ApproveReview(context.Context, uuid.UUID, string, string) (*model.Review, error)
	RequestChangesReview(context.Context, uuid.UUID, string, string) (*model.Review, error)
	RejectReview(context.Context, uuid.UUID, string, string, string) (*model.Review, error)
	MarkFalsePositive(context.Context, uuid.UUID, string, []string, string) (*model.Review, error)
	Approve(context.Context, uuid.UUID, string) (*model.Review, error)
	Reject(context.Context, uuid.UUID, string, string) (*model.Review, error)
	GetByID(context.Context, uuid.UUID) (*model.Review, error)
	GetByTask(context.Context, uuid.UUID) ([]*model.Review, error)
	ListAll(context.Context, string, string, int) ([]*model.Review, error)
	IngestCIResult(context.Context, *model.CIReviewRequest) (*model.Review, error)
	RequestHumanApproval(context.Context, uuid.UUID) error
} = (*ReviewService)(nil)

func (s *ReviewService) ApproveReview(ctx context.Context, id uuid.UUID, actor, comment string) (*model.Review, error) {
	return s.applyHumanTransition(ctx, id, actor, model.ReviewRecommendationApprove, strings.TrimSpace(comment))
}

func (s *ReviewService) RequestChangesReview(ctx context.Context, id uuid.UUID, actor, comment string) (*model.Review, error) {
	return s.applyHumanTransition(ctx, id, actor, model.ReviewRecommendationRequestChanges, strings.TrimSpace(comment))
}

func (s *ReviewService) RejectReview(ctx context.Context, id uuid.UUID, actor, reason, comment string) (*model.Review, error) {
	summary := reason
	if comment != "" {
		summary = reason + ": " + comment
	}
	return s.applyHumanTransition(ctx, id, actor, model.ReviewRecommendationReject, strings.TrimSpace(summary))
}

func (s *ReviewService) MarkFalsePositive(ctx context.Context, reviewID uuid.UUID, actor string, findingIDs []string, reason string) (*model.Review, error) {
	review, err := s.reviews.GetByID(ctx, reviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}
	if len(findingIDs) == 0 {
		return nil, fmt.Errorf("at least one finding id is required")
	}

	targets := make(map[string]struct{}, len(findingIDs))
	for _, findingID := range findingIDs {
		trimmed := strings.TrimSpace(findingID)
		if trimmed == "" {
			continue
		}
		targets[trimmed] = struct{}{}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one finding id is required")
	}

	updatedFindings := append([]model.ReviewFinding(nil), review.Findings...)
	matchedCount := 0
	for idx := range updatedFindings {
		finding := updatedFindings[idx]
		if _, ok := targets[finding.ID]; ok {
			updatedFindings[idx].Dismissed = true
			matchedCount++
			continue
		}
		if _, ok := targets[strconv.Itoa(idx)]; ok {
			updatedFindings[idx].Dismissed = true
			matchedCount++
		}
	}
	if matchedCount == 0 {
		return nil, fmt.Errorf("no findings matched the provided ids")
	}

	review.Findings = updatedFindings
	review.ExecutionMetadata = appendReviewDecision(review.ExecutionMetadata, model.ReviewDecision{
		Actor:     normalizeReviewActor(actor),
		Action:    "false_positive",
		Comment:   strings.TrimSpace(reason),
		Timestamp: time.Now().UTC(),
	})
	if err := s.reviews.UpdateResult(ctx, review); err != nil {
		return nil, fmt.Errorf("update review false-positive state: %w", err)
	}

	projectID, _ := s.resolveReviewProject(ctx, review)
	if projectID != uuid.Nil {
		s.broadcast(ctx, ws.EventReviewUpdated, projectID.String(), review.ToDTO())
	}
	return review, nil
}

func (s *ReviewService) Approve(ctx context.Context, id uuid.UUID, comment string) (*model.Review, error) {
	return s.ApproveReview(ctx, id, "api", comment)
}

func (s *ReviewService) Reject(ctx context.Context, id uuid.UUID, reason, comment string) (*model.Review, error) {
	return s.RejectReview(ctx, id, "api", reason, comment)
}

func (s *ReviewService) applyHumanTransition(
	ctx context.Context,
	reviewID uuid.UUID,
	actor string,
	recommendation string,
	comment string,
) (*model.Review, error) {
	review, err := s.reviews.GetByID(ctx, reviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}
	if review.Status != model.ReviewStatusPendingHuman {
		return nil, ErrReviewInvalidTransition
	}

	review.ExecutionMetadata = appendReviewDecision(review.ExecutionMetadata, model.ReviewDecision{
		Actor:     normalizeReviewActor(actor),
		Action:    recommendation,
		Comment:   comment,
		Timestamp: time.Now().UTC(),
	})
	review.Recommendation = recommendation
	review.Status = model.ReviewStatusCompleted
	if recommendation == model.ReviewRecommendationReject {
		review.Status = model.ReviewStatusFailed
	}

	if err := s.reviews.UpdateResult(ctx, review); err != nil {
		return nil, fmt.Errorf("update review transition result: %w", err)
	}

	projectID, task := s.resolveReviewProject(ctx, review)
	if task != nil {
		targetStatus := mapRecommendationToTaskStatus(recommendation)
		if targetStatus != "" {
			if err := s.transitionTaskForReview(ctx, task, targetStatus); err != nil {
				return nil, err
			}
		}
	}

	eventType := ws.EventReviewUpdated
	if recommendation == model.ReviewRecommendationApprove {
		eventType = ws.EventReviewCompleted
	}
	if projectID != uuid.Nil {
		s.broadcast(ctx, eventType, projectID.String(), review.ToDTO())
	}

	return review, nil
}

func appendReviewDecision(metadata *model.ReviewExecutionMetadata, decision model.ReviewDecision) *model.ReviewExecutionMetadata {
	cloned := model.CloneReviewExecutionMetadata(metadata)
	if cloned == nil {
		cloned = &model.ReviewExecutionMetadata{}
	}
	cloned.Decisions = append(cloned.Decisions, decision)
	return cloned
}

func normalizeReviewActor(actor string) string {
	trimmed := strings.TrimSpace(actor)
	if trimmed == "" {
		return "system"
	}
	return trimmed
}

func (s *ReviewService) resolveReviewProject(ctx context.Context, review *model.Review) (uuid.UUID, *model.Task) {
	if review == nil {
		return uuid.Nil, nil
	}
	if review.TaskID != uuid.Nil {
		task, err := s.tasks.GetByID(ctx, review.TaskID)
		if err == nil && task != nil {
			return task.ProjectID, task
		}
	}
	return projectIDFromMetadata(review.ExecutionMetadata), nil
}

func (s *ReviewService) GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error) {
	review, err := s.reviews.GetByID(ctx, id)
	if err != nil {
		return nil, ErrReviewNotFound
	}
	return review, nil
}

func (s *ReviewService) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error) {
	return s.reviews.GetByTask(ctx, taskID)
}

func (s *ReviewService) ListAll(ctx context.Context, status, riskLevel string, limit int) ([]*model.Review, error) {
	return s.reviews.ListAll(ctx, status, riskLevel, limit)
}

func (s *ReviewService) transitionTaskForReview(ctx context.Context, task *model.Task, targetStatus string) error {
	fields := log.Fields{
		"taskId":        task.ID.String(),
		"projectId":     task.ProjectID.String(),
		"currentStatus": task.Status,
		"targetStatus":  targetStatus,
	}
	if task.Status == targetStatus {
		return nil
	}
	if task.Status != model.TaskStatusInReview {
		if err := s.tasks.TransitionStatus(ctx, task.ID, model.TaskStatusInReview); err != nil {
			return fmt.Errorf("transition task to in_review: %w", err)
		}
		task.Status = model.TaskStatusInReview
		log.WithFields(fields).Info("review transitioned task to in_review")
	}
	if err := s.tasks.TransitionStatus(ctx, task.ID, targetStatus); err != nil {
		return fmt.Errorf("transition task to %s: %w", targetStatus, err)
	}
	task.Status = targetStatus
	fields["currentStatus"] = task.Status
	log.WithFields(fields).Info("review transitioned task to final status")
	return nil
}

func mapRecommendationToTaskStatus(recommendation string) string {
	switch recommendation {
	case model.ReviewRecommendationApprove:
		return model.TaskStatusDone
	case model.ReviewRecommendationRequestChanges:
		return model.TaskStatusChangesRequested
	case model.ReviewRecommendationReject:
		return model.TaskStatusCancelled
	default:
		return ""
	}
}

func (s *ReviewService) shouldRoutePendingHuman(ctx context.Context, projectID uuid.UUID, review *model.Review) (bool, string) {
	if projectID == uuid.Nil || review == nil || s.projects == nil {
		return false, ""
	}

	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil || project == nil {
		return false, ""
	}
	policy := project.StoredSettings().ReviewPolicy
	if policy.RequireManualApproval {
		return true, "manual_approval_required"
	}

	threshold := strings.TrimSpace(policy.MinRiskLevelForBlock)
	if threshold == "" {
		return false, ""
	}
	if meetsRiskThreshold(reviewMaxSeverity(review), threshold) {
		return true, "risk_threshold_exceeded"
	}
	return false, ""
}

func reviewMaxSeverity(review *model.Review) string {
	if review == nil {
		return ""
	}
	maxSeverity := strings.TrimSpace(review.RiskLevel)
	for _, finding := range review.Findings {
		if compareRiskSeverity(finding.Severity, maxSeverity) > 0 {
			maxSeverity = finding.Severity
		}
	}
	return strings.TrimSpace(maxSeverity)
}

func meetsRiskThreshold(actual, threshold string) bool {
	return compareRiskSeverity(actual, threshold) >= 0
}

func compareRiskSeverity(left, right string) int {
	leftScore := severityScore(left)
	rightScore := severityScore(right)
	switch {
	case leftScore > rightScore:
		return 1
	case leftScore < rightScore:
		return -1
	default:
		return 0
	}
}

func severityScore(level string) int {
	switch strings.TrimSpace(strings.ToLower(level)) {
	case model.ReviewRiskLevelCritical:
		return 4
	case model.ReviewRiskLevelHigh:
		return 3
	case model.ReviewRiskLevelMedium:
		return 2
	case model.ReviewRiskLevelLow:
		return 1
	default:
		return 0
	}
}

func projectIDFromMetadata(metadata ...*model.ReviewExecutionMetadata) uuid.UUID {
	for _, item := range metadata {
		if item == nil {
			continue
		}
		raw := strings.TrimSpace(item.ProjectID)
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err == nil {
			return id
		}
	}
	return uuid.Nil
}

func (s *ReviewService) broadcast(ctx context.Context, eventType, projectID string, payload any) {
	_ = eventbus.PublishLegacy(ctx, s.bus, eventType, projectID, payload)
}

func (s *ReviewService) recordProgress(ctx context.Context, taskID uuid.UUID, input TaskActivityInput) {
	if s.progress == nil {
		return
	}
	if _, err := s.progress.RecordActivity(ctx, taskID, input); err != nil {
		log.WithFields(log.Fields{
			"taskId":         taskID.String(),
			"source":         input.Source,
			"occurredAt":     input.OccurredAt.Format(time.RFC3339),
			"updateHealth":   input.UpdateHealth,
			"markTransition": input.MarkTransition,
		}).WithError(err).Warn("review progress recording failed")
	}
}

func (s *ReviewService) buildExecutionPlan(ctx context.Context, req *model.TriggerReviewRequest) (*model.ReviewExecutionPlan, error) {
	if s.planner == nil {
		return &model.ReviewExecutionPlan{
			TriggerEvent: deriveReviewTriggerEvent(req),
			ChangedFiles: normalizeChangedFiles(req),
			Dimensions:   normalizeReviewDimensions(req.Dimensions),
			Plugins:      []model.ReviewExecutionPlugin{},
		}, nil
	}
	return s.planner.BuildPlan(ctx, req)
}

func reviewPluginRequestsFromPlan(plan *model.ReviewExecutionPlan) []bridgeclient.ReviewPluginRequest {
	if plan == nil || len(plan.Plugins) == 0 {
		return nil
	}

	plugins := make([]bridgeclient.ReviewPluginRequest, 0, len(plan.Plugins))
	for _, plugin := range plan.Plugins {
		plugins = append(plugins, bridgeclient.ReviewPluginRequest{
			PluginID:     plugin.ID,
			Name:         plugin.Name,
			Entrypoint:   plugin.Entrypoint,
			SourceType:   string(plugin.SourceType),
			Transport:    plugin.Transport,
			Command:      plugin.Command,
			Args:         append([]string(nil), plugin.Args...),
			URL:          plugin.URL,
			Events:       append([]string(nil), plugin.Events...),
			FilePatterns: append([]string(nil), plugin.FilePatterns...),
			OutputFormat: plugin.OutputFormat,
		})
	}
	return plugins
}

func reviewExecutionMetadataFromBridge(plan *model.ReviewExecutionPlan, response *bridgeclient.ReviewResponse) *model.ReviewExecutionMetadata {
	if plan == nil && response == nil {
		return nil
	}

	metadata := &model.ReviewExecutionMetadata{}
	if plan != nil {
		metadata.TriggerEvent = plan.TriggerEvent
		metadata.ChangedFiles = append([]string(nil), plan.ChangedFiles...)
		metadata.Dimensions = append([]string(nil), plan.Dimensions...)
	}
	if response != nil && len(response.DimensionResults) > 0 {
		metadata.Results = make([]model.ReviewExecutionResult, 0, len(response.DimensionResults))
		for _, result := range response.DimensionResults {
			item := model.ReviewExecutionResult{
				ID:          result.Dimension,
				Kind:        model.ReviewExecutionKindBuiltinDimension,
				Status:      model.ReviewExecutionStatus(result.Status),
				DisplayName: result.DisplayName,
				Summary:     result.Summary,
				Error:       result.Error,
			}
			if result.SourceType == "plugin" || result.PluginID != "" {
				item.Kind = model.ReviewExecutionKindPlugin
				if result.PluginID != "" {
					item.ID = result.PluginID
				}
			}
			metadata.Results = append(metadata.Results, item)
		}
	}
	if metadata.TriggerEvent == "" && len(metadata.ChangedFiles) == 0 && len(metadata.Dimensions) == 0 && len(metadata.Results) == 0 {
		return nil
	}
	return metadata
}

func (s *ReviewService) writeBackReviewFindings(ctx context.Context, task *model.Task, review *model.Review) (string, string, error) {
	if s.links == nil || s.pages == nil || s.versions == nil || task == nil || review == nil {
		return "", "", nil
	}

	docLink, err := s.pickWritebackLink(ctx, task.ProjectID, task.ID)
	if err != nil || docLink == nil {
		return "", "", err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		page, err := s.pages.GetByID(ctx, docLink.TargetID)
		if err != nil {
			return docLink.TargetID.String(), "", err
		}
		version, err := s.createReviewSnapshot(ctx, page, review)
		if err != nil {
			return page.ID.String(), "", err
		}
		page.Content = appendReviewFindingsBlocks(page.Content, review)
		page.ContentText = extractPlainText(page.Content)
		page.UpdatedAt = time.Now().UTC()
		if err := s.pages.Update(ctx, page); err != nil {
			lastErr = err
			if errors.Is(err, ErrWikiPageConflict) {
				continue
			}
			return page.ID.String(), version.ID.String(), err
		}
		return page.ID.String(), version.ID.String(), nil
	}

	return docLink.TargetID.String(), "", lastErr
}

func (s *ReviewService) pickWritebackLink(ctx context.Context, projectID uuid.UUID, taskID uuid.UUID) (*model.EntityLink, error) {
	links, err := s.links.ListBySource(ctx, projectID, model.EntityTypeTask, taskID)
	if err != nil {
		return nil, err
	}
	var requirement *model.EntityLink
	var design *model.EntityLink
	for _, link := range links {
		if link.TargetType != model.EntityTypeWikiPage {
			continue
		}
		switch link.LinkType {
		case model.EntityLinkTypeRequirement:
			if requirement == nil {
				requirement = link
			}
		case model.EntityLinkTypeDesign:
			if design == nil {
				design = link
			}
		}
	}
	if requirement != nil {
		return requirement, nil
	}
	return design, nil
}

func (s *ReviewService) createReviewSnapshot(ctx context.Context, page *model.WikiPage, review *model.Review) (*model.PageVersion, error) {
	versions, err := s.versions.ListByPageID(ctx, page.ID)
	if err != nil {
		return nil, err
	}
	next := 1
	if len(versions) > 0 {
		next = versions[0].VersionNumber + 1
	}
	version := &model.PageVersion{
		ID:            uuid.New(),
		PageID:        page.ID,
		VersionNumber: next,
		Name:          fmt.Sprintf("Review v%d findings", next),
		Content:       page.Content,
		CreatedBy:     page.UpdatedBy,
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.versions.Create(ctx, version); err != nil {
		return nil, err
	}
	return version, nil
}

func appendReviewFindingsBlocks(raw string, review *model.Review) string {
	blocks := []map[string]any{}
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		_ = json.Unmarshal([]byte(trimmed), &blocks)
	}
	blocks = append(blocks,
		map[string]any{
			"id":      uuid.NewString(),
			"type":    "heading",
			"content": "Review Findings",
		},
		map[string]any{
			"id":      uuid.NewString(),
			"type":    "paragraph",
			"content": review.Summary,
		},
	)
	for _, finding := range review.Findings {
		blocks = append(blocks, map[string]any{
			"id":      uuid.NewString(),
			"type":    "paragraph",
			"content": fmt.Sprintf("[%s] %s", finding.Severity, finding.Message),
		})
	}
	encoded, err := json.Marshal(blocks)
	if err != nil {
		return raw
	}
	return string(encoded)
}
