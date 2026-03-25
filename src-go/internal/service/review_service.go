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
)

var (
	ErrReviewNotFound     = errors.New("review not found")
	ErrReviewTaskNotFound = errors.New("review task not found")
)

type ReviewRepository interface {
	Create(ctx context.Context, review *model.Review) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error)
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

	executionPlan, err := s.buildExecutionPlan(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build review execution plan: %w", err)
	}

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

	if task.Status == model.TaskStatusInProgress {
		if err := s.tasks.TransitionStatus(ctx, task.ID, model.TaskStatusInReview); err != nil {
			return nil, fmt.Errorf("transition task to in_review: %w", err)
		}
		task.Status = model.TaskStatusInReview
	}

	s.broadcast(ws.EventReviewCreated, task.ProjectID.String(), review.ToDTO())
	s.recordProgress(ctx, task.ID, TaskActivityInput{
		Source:         model.TaskProgressSourceReviewCreated,
		OccurredAt:     time.Now().UTC(),
		UpdateHealth:   true,
		MarkTransition: true,
	})

	if s.bridge == nil {
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
		return nil, fmt.Errorf("bridge review: %w", err)
	}

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

	if err := s.reviews.UpdateResult(ctx, review); err != nil {
		return nil, fmt.Errorf("update review result: %w", err)
	}

	targetStatus := mapRecommendationToTaskStatus(req.Recommendation)
	if targetStatus != "" {
		if err := s.transitionTaskForReview(ctx, task, targetStatus); err != nil {
			return nil, err
		}
	}

	if task.AssigneeID != nil && s.notifications != nil {
		payload, _ := json.Marshal(review.ToDTO())
		_, _ = s.notifications.Create(
			ctx,
			*task.AssigneeID,
			model.NotificationTypeReviewCompleted,
			"Deep review completed",
			fmt.Sprintf("Layer 2 review finished for task %s with recommendation %s", task.Title, review.Recommendation),
			string(payload),
		)
	}

	s.broadcast(ws.EventReviewCompleted, task.ProjectID.String(), review.ToDTO())
	s.recordProgress(ctx, task.ID, TaskActivityInput{
		Source:         model.TaskProgressSourceReviewComplete,
		OccurredAt:     time.Now().UTC(),
		UpdateHealth:   true,
		MarkTransition: true,
	})
	if s.imProgress != nil {
		_, _ = s.imProgress.QueueBoundProgress(ctx, IMBoundProgressRequest{
			TaskID:     task.ID.String(),
			ReviewID:   review.ID.String(),
			Kind:       IMDeliveryKindTerminal,
			Content:    fmt.Sprintf("代码审查已完成。\nReview: %s\n状态: %s\n建议: %s", review.ID.String(), review.Status, review.Recommendation),
			IsTerminal: true,
		})
	}
	return review, nil
}

var _ interface {
	Trigger(context.Context, *model.TriggerReviewRequest) (*model.Review, error)
	Complete(context.Context, uuid.UUID, *model.CompleteReviewRequest) (*model.Review, error)
	Approve(context.Context, uuid.UUID, string) (*model.Review, error)
	Reject(context.Context, uuid.UUID, string, string) (*model.Review, error)
	GetByID(context.Context, uuid.UUID) (*model.Review, error)
	GetByTask(context.Context, uuid.UUID) ([]*model.Review, error)
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

func (s *ReviewService) transitionTaskForReview(ctx context.Context, task *model.Task, targetStatus string) error {
	if task.Status == targetStatus {
		return nil
	}
	if task.Status != model.TaskStatusInReview {
		if err := s.tasks.TransitionStatus(ctx, task.ID, model.TaskStatusInReview); err != nil {
			return fmt.Errorf("transition task to in_review: %w", err)
		}
		task.Status = model.TaskStatusInReview
	}
	if err := s.tasks.TransitionStatus(ctx, task.ID, targetStatus); err != nil {
		return fmt.Errorf("transition task to %s: %w", targetStatus, err)
	}
	task.Status = targetStatus
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
	_, _ = s.progress.RecordActivity(ctx, taskID, input)
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
