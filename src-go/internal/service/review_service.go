package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

var (
	ErrReviewNotFound     = errors.New("review not found")
	ErrReviewTaskNotFound = errors.New("review task not found")
)

type ReviewRepository interface {
	Create(ctx context.Context, review *model.Review) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error)
	ListAll(ctx context.Context, status, riskLevel string, limit int) ([]*model.Review, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateResult(ctx context.Context, review *model.Review) error
}

type ReviewTaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	GetByPRURL(ctx context.Context, prURL string) (*model.Task, error)
	TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
}

type ReviewNotificationCreator interface {
	Create(ctx context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error)
}

type ReviewBridgeClient interface {
	Review(ctx context.Context, req bridgeclient.ReviewRequest) (*bridgeclient.ReviewResponse, error)
}

type ReviewService struct {
	reviews       ReviewRepository
	tasks         ReviewTaskRepository
	notifications ReviewNotificationCreator
	hub           *ws.Hub
	bridge        ReviewBridgeClient
	planner       *ReviewExecutionPlanner
	progress      *TaskProgressService
	imProgress    IMBoundProgressNotifier
	aggregation   *ReviewAggregationService
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

func (s *ReviewService) WithAggregationService(agg *ReviewAggregationService) *ReviewService {
	s.aggregation = agg
	return s
}

func (s *ReviewService) Trigger(ctx context.Context, req *model.TriggerReviewRequest) (*model.Review, error) {
	var (
		task *model.Task
		err  error
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
	if err != nil || task == nil {
		return nil, ErrReviewTaskNotFound
	}
	triggerFields := log.Fields{
		"taskId":     task.ID.String(),
		"projectId":  task.ProjectID.String(),
		"taskStatus": task.Status,
		"trigger":    req.Trigger,
		"prUrl":      req.PRURL,
		"prNumber":   req.PRNumber,
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
		TaskID:    task.ID,
		PRURL:     req.PRURL,
		PRNumber:  req.PRNumber,
		Layer:     model.ReviewLayerDeep,
		Status:    model.ReviewStatusInProgress,
		RiskLevel: model.ReviewRiskLevelLow,
	}

	if err := s.reviews.Create(ctx, review); err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}
	triggerFields["reviewId"] = review.ID.String()
	log.WithFields(triggerFields).Info("review created")

	if task.Status == model.TaskStatusInProgress {
		if err := s.tasks.TransitionStatus(ctx, task.ID, model.TaskStatusInReview); err != nil {
			return nil, fmt.Errorf("transition task to in_review: %w", err)
		}
		task.Status = model.TaskStatusInReview
		log.WithFields(triggerFields).Info("task transitioned to in_review for review")
	}

	s.broadcast(ws.EventReviewCreated, task.ProjectID.String(), review.ToDTO())
	s.recordProgress(ctx, task.ID, TaskActivityInput{
		Source:         model.TaskProgressSourceReviewCreated,
		OccurredAt:     time.Now().UTC(),
		UpdateHealth:   true,
		MarkTransition: true,
	})

	if s.bridge == nil {
		log.WithFields(triggerFields).Debug("review trigger completed without bridge client")
		return review, nil
	}

	result, err := s.bridge.Review(ctx, bridgeclient.ReviewRequest{
		ReviewID:      review.ID.String(),
		TaskID:        task.ID.String(),
		PRURL:         req.PRURL,
		PRNumber:      req.PRNumber,
		Title:         task.Title,
		Description:   task.Description,
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

	return s.Complete(ctx, review.ID, &model.CompleteReviewRequest{
		RiskLevel:         result.RiskLevel,
		Findings:          result.Findings,
		ExecutionMetadata: reviewExecutionMetadataFromBridge(executionPlan, result),
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

	task, err := s.tasks.GetByID(ctx, review.TaskID)
	if err != nil {
		return nil, ErrReviewTaskNotFound
	}

	review.Status = model.ReviewStatusCompleted
	review.RiskLevel = req.RiskLevel
	review.Findings = req.Findings
	review.ExecutionMetadata = model.CloneReviewExecutionMetadata(req.ExecutionMetadata)
	review.Summary = req.Summary
	review.Recommendation = req.Recommendation
	review.CostUSD = req.CostUSD
	fields := reviewLogFields(review, task)
	fields["findingCount"] = len(req.Findings)
	fields["costUsd"] = req.CostUSD

	if err := s.reviews.UpdateResult(ctx, review); err != nil {
		return nil, fmt.Errorf("update review result: %w", err)
	}
	log.WithFields(fields).Info("review result stored")

	targetStatus := mapRecommendationToTaskStatus(req.Recommendation)
	if targetStatus != "" {
		if err := s.transitionTaskForReview(ctx, task, targetStatus); err != nil {
			log.WithFields(fields).WithError(err).Warn("review task transition failed")
			return nil, err
		}
		fields["taskStatus"] = task.Status
	}

	if task.AssigneeID != nil && s.notifications != nil {
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

	s.broadcast(ws.EventReviewCompleted, task.ProjectID.String(), review.ToDTO())
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
	// Trigger aggregation if service is available.
	if s.aggregation != nil {
		if agg, aggErr := s.aggregation.Aggregate(ctx, review.TaskID); aggErr != nil {
			log.WithFields(fields).WithError(aggErr).Warn("post-complete aggregation failed")
		} else {
			log.WithFields(fields).WithField("aggregationId", agg.ID.String()).Info("post-complete aggregation succeeded")
		}
	}

	log.WithFields(fields).Info("review completed")
	return review, nil
}

// IngestCIResult creates a review with Layer "ci" (Layer 1) from CI pipeline findings.
func (s *ReviewService) IngestCIResult(ctx context.Context, req *model.CIReviewRequest) (*model.Review, error) {
	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task id: %w", err)
	}

	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil || task == nil {
		return nil, ErrReviewTaskNotFound
	}

	// Determine risk level from CI status.
	riskLevel := model.ReviewRiskLevelLow
	recommendation := model.ReviewRecommendationApprove
	if req.Status == "failure" || req.Status == "error" {
		riskLevel = model.ReviewRiskLevelHigh
		recommendation = model.ReviewRecommendationRequestChanges
	}
	if len(req.Findings) > 0 {
		riskLevel = model.ReviewRiskLevelMedium
		recommendation = model.ReviewRecommendationRequestChanges
	}

	summary := fmt.Sprintf("CI result from %s: status=%s, findings=%d", req.CISystem, req.Status, len(req.Findings))

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
	log.WithFields(fields).Info("CI review ingested")

	s.broadcast(ws.EventReviewCreated, task.ProjectID.String(), review.ToDTO())

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

	task, err := s.tasks.GetByID(ctx, review.TaskID)
	if err != nil {
		return ErrReviewTaskNotFound
	}

	if err := s.reviews.UpdateStatus(ctx, reviewID, model.ReviewStatusPendingHuman); err != nil {
		return fmt.Errorf("update review to pending_human: %w", err)
	}

	review.Status = model.ReviewStatusPendingHuman
	fields := reviewLogFields(review, task)
	log.WithFields(fields).Info("review set to pending_human")

	s.broadcast(ws.EventReviewPendingHuman, task.ProjectID.String(), review.ToDTO())

	if task.AssigneeID != nil && s.notifications != nil {
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

// RouteFixRequest broadcasts an event with review findings formatted as fix
// instructions for an active agent to pick up.
func (s *ReviewService) RouteFixRequest(ctx context.Context, reviewID uuid.UUID) error {
	review, err := s.reviews.GetByID(ctx, reviewID)
	if err != nil {
		return ErrReviewNotFound
	}

	task, err := s.tasks.GetByID(ctx, review.TaskID)
	if err != nil {
		return ErrReviewTaskNotFound
	}

	fixPayload := map[string]any{
		"reviewId":       review.ID.String(),
		"taskId":         task.ID.String(),
		"recommendation": review.Recommendation,
		"findings":       review.Findings,
		"summary":        review.Summary,
	}

	fields := reviewLogFields(review, task)
	log.WithFields(fields).Info("routing fix request to agent")

	s.broadcast(ws.EventReviewFixRequested, task.ProjectID.String(), fixPayload)

	return nil
}

var _ interface {
	Trigger(context.Context, *model.TriggerReviewRequest) (*model.Review, error)
	Complete(context.Context, uuid.UUID, *model.CompleteReviewRequest) (*model.Review, error)
	Approve(context.Context, uuid.UUID, string) (*model.Review, error)
	Reject(context.Context, uuid.UUID, string, string) (*model.Review, error)
	GetByID(context.Context, uuid.UUID) (*model.Review, error)
	GetByTask(context.Context, uuid.UUID) ([]*model.Review, error)
	ListAll(context.Context, string, string, int) ([]*model.Review, error)
	IngestCIResult(context.Context, *model.CIReviewRequest) (*model.Review, error)
	RequestHumanApproval(context.Context, uuid.UUID) error
	RouteFixRequest(context.Context, uuid.UUID) error
} = (*ReviewService)(nil)

func (s *ReviewService) Approve(ctx context.Context, id uuid.UUID, comment string) (*model.Review, error) {
	return s.Complete(ctx, id, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelLow,
		Summary:        comment,
		Recommendation: model.ReviewRecommendationApprove,
	})
}

func (s *ReviewService) Reject(ctx context.Context, id uuid.UUID, reason, comment string) (*model.Review, error) {
	summary := reason
	if comment != "" {
		summary = reason + ": " + comment
	}
	return s.Complete(ctx, id, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelHigh,
		Summary:        summary,
		Recommendation: model.ReviewRecommendationReject,
	})
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

func (s *ReviewService) broadcast(eventType, projectID string, payload any) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      eventType,
		ProjectID: projectID,
		Payload:   payload,
	})
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
