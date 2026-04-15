package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type IMActionExecutor interface {
	Execute(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error)
}

type IMActionExecutorFunc func(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error)

func (fn IMActionExecutorFunc) Execute(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error) {
	return fn(ctx, req)
}

type IMActionTaskDispatcher interface {
	Assign(ctx context.Context, taskID uuid.UUID, req *model.AssignRequest) (*model.TaskDispatchResponse, error)
	Spawn(ctx context.Context, input DispatchSpawnInput) (*model.TaskDispatchResponse, error)
}

type IMActionTaskDecomposer interface {
	Decompose(ctx context.Context, taskID uuid.UUID) (*model.TaskDecompositionResponse, error)
}

type IMActionReviewer interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
	ApproveReview(ctx context.Context, id uuid.UUID, actor, comment string) (*model.Review, error)
	RequestChangesReview(ctx context.Context, id uuid.UUID, actor, comment string) (*model.Review, error)
	RouteFixRequest(ctx context.Context, id uuid.UUID) error
}

type IMActionTaskCreator interface {
	Create(ctx context.Context, projectID uuid.UUID, req *model.CreateTaskRequest, reporterID *uuid.UUID) (*model.Task, error)
}

type IMActionTaskTransitioner interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	Transition(ctx context.Context, id uuid.UUID, req *model.TransitionRequest) (*model.Task, error)
}

type IMActionBindingWriter interface {
	BindAction(ctx context.Context, binding *model.IMActionBinding) error
}

type IMActionProgressRecorder interface {
	RecordActivity(ctx context.Context, taskID uuid.UUID, input TaskActivityInput) (*model.TaskProgressSnapshot, error)
}

type IMActionWorkflowEvaluator interface {
	EvaluateTransition(ctx context.Context, task *model.Task, fromStatus, toStatus string) []TriggerResult
}

type IMActionWikiCreator interface {
	GetSpaceByProjectID(ctx context.Context, projectID uuid.UUID) (*model.WikiSpace, error)
	CreatePage(ctx context.Context, projectID uuid.UUID, spaceID uuid.UUID, title string, parentID *uuid.UUID, content string, createdBy *uuid.UUID) (*model.WikiPage, error)
}

type IMReactionRecorder interface {
	Record(ctx context.Context, event *model.IMReactionEvent) error
}

type BackendIMActionExecutor struct {
	dispatcher IMActionTaskDispatcher
	decomposer IMActionTaskDecomposer
	reviewer   IMActionReviewer
	taskMaker  IMActionTaskCreator
	taskMover  IMActionTaskTransitioner
	binder     IMActionBindingWriter
	progress   IMActionProgressRecorder
	workflow   IMActionWorkflowEvaluator
	wikiMaker  IMActionWikiCreator
	reactions  IMReactionRecorder
}

func NewBackendIMActionExecutor(dispatcher IMActionTaskDispatcher, decomposer IMActionTaskDecomposer, reviewer IMActionReviewer, extras ...any) *BackendIMActionExecutor {
	executor := &BackendIMActionExecutor{
		dispatcher: dispatcher,
		decomposer: decomposer,
		reviewer:   reviewer,
	}
	for _, extra := range extras {
		switch value := extra.(type) {
		case IMActionTaskCreator:
			executor.taskMaker = value
		case IMActionTaskTransitioner:
			executor.taskMover = value
		case IMActionBindingWriter:
			executor.binder = value
		case IMActionProgressRecorder:
			executor.progress = value
		case IMActionWorkflowEvaluator:
			executor.workflow = value
		case IMActionWikiCreator:
			executor.wikiMaker = value
		case IMReactionRecorder:
			executor.reactions = value
		}
	}
	return executor
}

func (e *BackendIMActionExecutor) Execute(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error) {
	if req == nil {
		return nil, nil
	}

	switch strings.TrimSpace(req.Action) {
	case "assign-agent":
		return e.executeAssignAgent(ctx, req), nil
	case "decompose":
		return e.executeDecompose(ctx, req), nil
	case "transition-task", "move-task":
		return e.executeTransitionTask(ctx, req), nil
	case "save-as-doc":
		return e.executeSaveAsDoc(ctx, req), nil
	case "create-task":
		return e.executeCreateTask(ctx, req), nil
	case "approve":
		return e.executeReviewAction(ctx, req, model.ReviewRecommendationApprove), nil
	case "request-changes":
		return e.executeReviewAction(ctx, req, model.ReviewRecommendationRequestChanges), nil
	case "react":
		return e.executeReact(ctx, req), nil
	default:
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Unknown action: %s", req.Action), false), nil
	}
}

func (e *BackendIMActionExecutor) executeAssignAgent(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	taskID, err := parseIMEntityUUID(req.EntityID)
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Invalid task identifier.", false)
	}
	if e.dispatcher == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Task dispatch workflow is unavailable.", false)
	}

	metadata := cloneStringMap(req.Metadata)
	var result *model.TaskDispatchResponse
	assigneeID := firstMetadataValue(metadata, "assigneeId", "assignee_id", "memberId", "member_id")
	if assigneeID != "" {
		assignReq := &model.AssignRequest{
			AssigneeID:    assigneeID,
			AssigneeType:  firstNonEmptyMetadata(metadata, model.MemberTypeAgent, "assigneeType", "assignee_type"),
			TriggerSource: "im",
		}
		result, err = e.dispatcher.Assign(ctx, taskID, assignReq)
	} else {
		result, err = e.dispatcher.Spawn(ctx, DispatchSpawnInput{TaskID: taskID, TriggerSource: "im"})
	}
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Agent dispatch failed: %s", err.Error()), false)
	}
	if result == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Agent dispatch returned no result.", false)
	}

	resp := newIMActionResponse(req, mapDispatchStatusToIMActionStatus(result.Dispatch.Status), describeDispatchOutcome(result), dispatchOutcomeSuccessful(result.Dispatch.Status))
	resp.Dispatch = cloneDispatchOutcomePtr(&result.Dispatch)
	if result.Task.ID != "" {
		task := result.Task
		resp.Task = &task
		attachTaskBindingMetadata(resp, req, task.ProjectID, task.ID)
	}
	return resp
}

func (e *BackendIMActionExecutor) executeSaveAsDoc(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	projectID, err := parseIMEntityUUID(req.EntityID)
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Invalid project identifier.", false)
	}
	if e.wikiMaker == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Wiki workflow is unavailable.", false)
	}
	space, err := e.wikiMaker.GetSpaceByProjectID(ctx, projectID)
	if err != nil || space == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Wiki space not found.", false)
	}
	title := firstNonEmptyMetadata(req.Metadata, "IM Note", "title", "name")
	body := firstNonEmptyMetadata(req.Metadata, "", "body", "text", "content")
	page, err := e.wikiMaker.CreatePage(ctx, projectID, space.ID, title, nil, body, nil)
	if err != nil || page == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Failed to save message as doc.", false)
	}
	resp := newIMActionResponse(req, model.IMActionStatusCompleted, fmt.Sprintf("Saved message as doc %s.", page.Title), true)
	resp.Metadata["href"] = "/docs/" + page.ID.String()
	return resp
}

func (e *BackendIMActionExecutor) executeCreateTask(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	projectID, err := parseIMEntityUUID(req.EntityID)
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Invalid project identifier.", false)
	}
	if e.taskMaker == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Task creation workflow is unavailable.", false)
	}
	title := firstNonEmptyMetadata(req.Metadata, "IM Task", "title", "name")
	description := firstNonEmptyMetadata(req.Metadata, "", "body", "text", "content")
	priority := firstNonEmptyMetadata(req.Metadata, "medium", "priority")
	task, err := e.taskMaker.Create(ctx, projectID, &model.CreateTaskRequest{
		Title:       title,
		Description: description,
		Priority:    priority,
	}, nil)
	if err != nil || task == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Failed to create task from message.", false)
	}
	resp := newIMActionResponse(req, model.IMActionStatusCompleted, fmt.Sprintf("Created task %s.", task.Title), true)
	dto := task.ToDTO()
	resp.Task = &dto
	attachTaskBindingMetadata(resp, req, dto.ProjectID, dto.ID)
	resp.Metadata["href"] = "/project?taskId=" + task.ID.String()
	return resp
}

func (e *BackendIMActionExecutor) executeDecompose(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	taskID, err := parseIMEntityUUID(req.EntityID)
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Invalid task identifier.", false)
	}
	if e.decomposer == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Task decomposition workflow is unavailable.", false)
	}

	result, err := e.decomposer.Decompose(ctx, taskID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskAlreadyDecomposed):
			return newIMActionResponse(req, model.IMActionStatusBlocked, "Task already has child tasks and cannot be decomposed again.", false)
		case errors.Is(err, ErrTaskNotFound):
			return newIMActionResponse(req, model.IMActionStatusFailed, "Task not found.", false)
		default:
			return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Task decomposition failed: %s", err.Error()), false)
		}
	}
	if result == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Task decomposition returned no result.", false)
	}

	resp := newIMActionResponse(req, model.IMActionStatusCompleted, fmt.Sprintf("Task %s was decomposed into %d subtasks.", result.ParentTask.ID, len(result.Subtasks)), true)
	resp.Task = cloneTaskDTOPtr(&result.ParentTask)
	resp.Decomposition = cloneTaskDecompositionResponse(result)
	if resp.Task != nil {
		attachTaskBindingMetadata(resp, req, resp.Task.ProjectID, resp.Task.ID)
	}
	return resp
}

func (e *BackendIMActionExecutor) executeTransitionTask(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	taskID, err := parseIMEntityUUID(req.EntityID)
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Invalid task identifier.", false)
	}
	if e.taskMover == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Task transition workflow is unavailable.", false)
	}

	targetStatus := firstMetadataValue(req.Metadata, "targetStatus", "target_status", "status")
	if targetStatus == "" {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Task transition requires target status.", false)
	}

	currentTask, currentErr := e.taskMover.GetByID(ctx, taskID)
	updatedTask, err := e.taskMover.Transition(ctx, taskID, &model.TransitionRequest{Status: targetStatus})
	if err != nil {
		status := model.IMActionStatusFailed
		message := fmt.Sprintf("Task transition failed: %s", err.Error())
		if currentTask != nil {
			status = model.IMActionStatusBlocked
			message = fmt.Sprintf("Task %s could not transition to %s: %s", currentTask.Title, targetStatus, err.Error())
		} else if currentErr != nil {
			message = fmt.Sprintf("Task transition failed: %s", currentErr.Error())
		}
		resp := newIMActionResponse(req, status, message, false)
		if currentTask != nil {
			dto := currentTask.ToDTO()
			resp.Task = &dto
			attachTaskBindingMetadata(resp, req, dto.ProjectID, dto.ID)
		}
		return resp
	}

	if updatedTask == nil {
		updatedTask, _ = e.taskMover.GetByID(ctx, taskID)
	}
	if updatedTask == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Task transition returned no task.", false)
	}

	e.bindTaskAction(ctx, req, updatedTask)
	e.recordTaskTransitionActivity(ctx, updatedTask)
	e.evaluateTaskTransitionWorkflow(ctx, currentTask, updatedTask)

	dto := updatedTask.ToDTO()
	resp := newIMActionResponse(req, model.IMActionStatusCompleted, fmt.Sprintf("Task %s moved to %s.", updatedTask.Title, dto.Status), true)
	resp.Task = &dto
	resp.Structured = buildTaskLifecycleStructuredMessage(dto)
	attachTaskBindingMetadata(resp, req, dto.ProjectID, dto.ID)
	return resp
}

func (e *BackendIMActionExecutor) executeReviewAction(ctx context.Context, req *model.IMActionRequest, recommendation string) *model.IMActionResponse {
	reviewID, err := parseIMEntityUUID(req.EntityID)
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Invalid review identifier.", false)
	}
	if e.reviewer == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Review workflow is unavailable.", false)
	}

	review, err := e.reviewer.GetByID(ctx, reviewID)
	if err != nil || review == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Review not found.", false)
	}
	if reason := reviewActionBlockReason(review, recommendation); reason != "" {
		return newIMActionResponse(req, model.IMActionStatusBlocked, reason, false)
	}

	var updated *model.Review
	actor := firstNonEmptyMetadata(req.Metadata, "im-action", "actor", "userId", "user_id")
	switch recommendation {
	case model.ReviewRecommendationApprove:
		updated, err = e.reviewer.ApproveReview(ctx, reviewID, actor, firstMetadataValue(req.Metadata, "comment", "notes"))
	case model.ReviewRecommendationRequestChanges:
		updated, err = e.reviewer.RequestChangesReview(ctx, reviewID, actor, firstMetadataValue(req.Metadata, "comment", "notes"))
		if err == nil {
			if routeErr := e.reviewer.RouteFixRequest(ctx, reviewID); routeErr != nil {
				metadata := cloneStringMap(req.Metadata)
				metadata["route_fix_error"] = routeErr.Error()
				resp := newIMActionResponse(req, model.IMActionStatusCompleted, fmt.Sprintf("Review %s marked as request_changes, but fix routing failed: %s", reviewID.String(), routeErr.Error()), true)
				resp.Metadata = metadata
				if updated != nil {
					dto := updated.ToDTO()
					resp.Review = &dto
				}
				return resp
			}
		}
	default:
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Unsupported review action: %s", recommendation), false)
	}
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Review action failed: %s", err.Error()), false)
	}
	if updated == nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Review action returned no result.", false)
	}

	resp := newIMActionResponse(req, model.IMActionStatusCompleted, describeReviewOutcome(updated), true)
	dto := updated.ToDTO()
	resp.Review = &dto
	return resp
}

func (e *BackendIMActionExecutor) executeReact(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	if e.reactions == nil {
		return newIMActionResponse(req, model.IMActionStatusBlocked, "Reaction recording is not configured.", false)
	}
	messageID := strings.TrimSpace(req.EntityID)
	if messageID == "" {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Reaction event missing message id.", false)
	}
	eventType := strings.TrimSpace(req.Metadata["event_type"])
	if eventType == "" {
		eventType = model.IMReactionEventTypeCreated
	}
	if eventType != model.IMReactionEventTypeCreated && eventType != model.IMReactionEventTypeDeleted {
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Unknown reaction event type %q.", eventType), false)
	}
	event := &model.IMReactionEvent{
		Platform:  strings.TrimSpace(req.Platform),
		ChatID:    strings.TrimSpace(req.ChannelID),
		MessageID: messageID,
		UserID:    strings.TrimSpace(req.UserID),
		Emoji:     strings.TrimSpace(req.Metadata["emoji"]),
		EventType: eventType,
	}
	if err := e.reactions.Record(ctx, event); err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Record reaction failed: %s", err.Error()), false)
	}
	return newIMActionResponse(req, model.IMActionStatusCompleted, "", true)
}

func attachTaskBindingMetadata(resp *model.IMActionResponse, req *model.IMActionRequest, projectID string, taskID string) {
	if resp == nil {
		return
	}
	resp.Metadata = buildIMConnectivityMetadata(
		resp.Metadata,
		imDeliverySourceActionResult,
		req.BridgeID,
		req.Platform,
		projectID,
		taskID,
		"",
		"",
		resp.ReplyTarget,
	)
	if trimmed := strings.TrimSpace(resp.Status); trimmed != "" {
		resp.Metadata["action_status"] = trimmed
	}
}

func (e *BackendIMActionExecutor) bindTaskAction(ctx context.Context, req *model.IMActionRequest, task *model.Task) {
	if e == nil || e.binder == nil || req == nil || req.ReplyTarget == nil || task == nil {
		return
	}
	_ = e.binder.BindAction(ctx, &model.IMActionBinding{
		BridgeID:    strings.TrimSpace(req.BridgeID),
		Platform:    req.Platform,
		ProjectID:   task.ProjectID.String(),
		TaskID:      task.ID.String(),
		ReplyTarget: cloneReplyTarget(req.ReplyTarget),
	})
}

func (e *BackendIMActionExecutor) recordTaskTransitionActivity(ctx context.Context, task *model.Task) {
	if e == nil || e.progress == nil || task == nil {
		return
	}
	_, _ = e.progress.RecordActivity(ctx, task.ID, TaskActivityInput{
		Source:         model.TaskProgressSourceTaskTransition,
		UpdateHealth:   true,
		MarkTransition: true,
		OccurredAt:     task.UpdatedAt,
	})
}

func (e *BackendIMActionExecutor) evaluateTaskTransitionWorkflow(ctx context.Context, previousTask *model.Task, updatedTask *model.Task) {
	if e == nil || e.workflow == nil || previousTask == nil || updatedTask == nil {
		return
	}
	if strings.TrimSpace(previousTask.Status) == "" || strings.TrimSpace(updatedTask.Status) == "" {
		return
	}
	e.workflow.EvaluateTransition(ctx, updatedTask, previousTask.Status, updatedTask.Status)
}

func buildTaskLifecycleStructuredMessage(task model.TaskDTO) *model.IMStructuredMessage {
	fields := []model.IMStructuredField{
		{Label: "Task", Value: task.Title},
		{Label: "Status", Value: task.Status},
		{Label: "Priority", Value: task.Priority},
	}
	if nextStatus, ok := nextTaskLifecycleStatus(task.Status); ok {
		fields = append(fields, model.IMStructuredField{Label: "Next", Value: nextStatus})
	}

	actions := []model.IMStructuredAction{
		{Label: "View Task", URL: "/tasks/" + task.ID, Style: "default"},
	}
	if nextStatus, ok := nextTaskLifecycleStatus(task.Status); ok {
		actions = append([]model.IMStructuredAction{{
			ID:    "act:transition-task:" + task.ID + "?targetStatus=" + nextStatus,
			Label: nextTaskLifecycleLabel(nextStatus),
			Style: "primary",
		}}, actions...)
	}

	return &model.IMStructuredMessage{
		Title:   "Task Update",
		Body:    fmt.Sprintf("Task %s is now %s.", task.Title, task.Status),
		Fields:  fields,
		Actions: actions,
	}
}

func nextTaskLifecycleStatus(status string) (string, bool) {
	switch strings.TrimSpace(status) {
	case model.TaskStatusInbox:
		return model.TaskStatusTriaged, true
	case model.TaskStatusTriaged:
		return model.TaskStatusAssigned, true
	case model.TaskStatusAssigned:
		return model.TaskStatusInProgress, true
	case model.TaskStatusInProgress:
		return model.TaskStatusInReview, true
	case model.TaskStatusInReview:
		return model.TaskStatusDone, true
	case model.TaskStatusChangesRequested:
		return model.TaskStatusInProgress, true
	case model.TaskStatusBlocked:
		return model.TaskStatusInProgress, true
	case model.TaskStatusBudgetExceeded:
		return model.TaskStatusInProgress, true
	default:
		return "", false
	}
}

func nextTaskLifecycleLabel(status string) string {
	switch strings.TrimSpace(status) {
	case model.TaskStatusTriaged:
		return "Mark Triaged"
	case model.TaskStatusAssigned:
		return "Mark Assigned"
	case model.TaskStatusInProgress:
		return "Start Work"
	case model.TaskStatusInReview:
		return "Request Review"
	case model.TaskStatusDone:
		return "Mark Done"
	default:
		return "Transition Task"
	}
}

func newIMActionResponse(req *model.IMActionRequest, status string, message string, success bool) *model.IMActionResponse {
	return &model.IMActionResponse{
		Result:      strings.TrimSpace(message),
		Success:     success,
		Status:      strings.TrimSpace(status),
		ReplyTarget: cloneReplyTarget(req.ReplyTarget),
		Metadata:    buildIMActionResponseMetadata(req, status),
	}
}

func describeDispatchOutcome(result *model.TaskDispatchResponse) string {
	if result == nil {
		return "Agent dispatch did not return a result."
	}
	taskID := strings.TrimSpace(result.Task.ID)
	switch result.Dispatch.Status {
	case model.DispatchStatusStarted:
		if result.Dispatch.Run != nil {
			return fmt.Sprintf("Task %s was dispatched and agent run %s started.", taskID, result.Dispatch.Run.ID)
		}
		return fmt.Sprintf("Task %s was dispatched to an agent.", taskID)
	case model.DispatchStatusQueued:
		if result.Dispatch.Queue != nil && result.Dispatch.Queue.Priority > 0 {
			return fmt.Sprintf("Task %s entered the agent queue at priority %d: %s", taskID, result.Dispatch.Queue.Priority, defaultDispatchReason(result.Dispatch.Reason, "agent pool is at capacity"))
		}
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("Task %s entered the agent queue: %s", taskID, reason)
		}
		return fmt.Sprintf("Task %s entered the agent queue.", taskID)
	case model.DispatchStatusBlocked:
		if result.Dispatch.GuardrailType == model.DispatchGuardrailTypeBudget {
			if scope := strings.TrimSpace(result.Dispatch.GuardrailScope); scope != "" {
				return fmt.Sprintf("Task %s could not start an agent because the %s budget blocked dispatch: %s", taskID, scope, defaultDispatchReason(result.Dispatch.Reason, "budget guardrail"))
			}
		}
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("Task %s could not start an agent: %s", taskID, reason)
		}
		return fmt.Sprintf("Task %s could not start an agent.", taskID)
	case model.DispatchStatusSkipped:
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("Task %s dispatch was skipped: %s", taskID, reason)
		}
		return fmt.Sprintf("Task %s dispatch was skipped.", taskID)
	default:
		return fmt.Sprintf("Task %s dispatch returned status %s.", taskID, result.Dispatch.Status)
	}
}

func describeReviewOutcome(review *model.Review) string {
	if review == nil {
		return "Review action completed."
	}
	switch review.Recommendation {
	case model.ReviewRecommendationApprove:
		return fmt.Sprintf("Review %s was approved.", review.ID.String())
	case model.ReviewRecommendationRequestChanges:
		return fmt.Sprintf("Review %s now requires changes.", review.ID.String())
	default:
		return fmt.Sprintf("Review %s updated to %s.", review.ID.String(), review.Recommendation)
	}
}

func reviewActionBlockReason(review *model.Review, recommendation string) string {
	if review == nil {
		return "Review not found."
	}
	if recommendation == model.ReviewRecommendationApprove || recommendation == model.ReviewRecommendationRequestChanges {
		if review.Status != model.ReviewStatusPendingHuman {
			return fmt.Sprintf("Review %s is in %s state; only pending_human reviews can be updated interactively.", review.ID.String(), review.Status)
		}
	}
	if review.Status == model.ReviewStatusCompleted {
		if review.Recommendation == recommendation {
			return fmt.Sprintf("Review %s is already completed as %s.", review.ID.String(), review.Recommendation)
		}
		return fmt.Sprintf("Review %s is already completed as %s and cannot transition to %s.", review.ID.String(), review.Recommendation, recommendation)
	}
	if review.Status == model.ReviewStatusFailed {
		return fmt.Sprintf("Review %s is in failed state and cannot accept interactive updates.", review.ID.String())
	}
	return ""
}

func parseIMEntityUUID(raw string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(raw))
}

func mapDispatchStatusToIMActionStatus(status string) string {
	switch strings.TrimSpace(status) {
	case model.DispatchStatusStarted, model.DispatchStatusQueued:
		return model.IMActionStatusStarted
	case model.DispatchStatusBlocked:
		return model.IMActionStatusBlocked
	case model.DispatchStatusSkipped:
		return model.IMActionStatusCompleted
	default:
		return model.IMActionStatusFailed
	}
}

func dispatchOutcomeSuccessful(status string) bool {
	switch strings.TrimSpace(status) {
	case model.DispatchStatusStarted, model.DispatchStatusQueued, model.DispatchStatusSkipped:
		return true
	default:
		return false
	}
}

func firstMetadataValue(metadata map[string]string, keys ...string) string {
	for _, key := range keys {
		if metadata == nil {
			continue
		}
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyMetadata(metadata map[string]string, fallback string, keys ...string) string {
	if value := firstMetadataValue(metadata, keys...); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func cloneTaskDTOPtr(task *model.TaskDTO) *model.TaskDTO {
	if task == nil {
		return nil
	}
	clone := *task
	clone.Labels = append([]string(nil), task.Labels...)
	clone.BlockedBy = append([]string(nil), task.BlockedBy...)
	return &clone
}

func cloneDispatchOutcomePtr(outcome *model.DispatchOutcome) *model.DispatchOutcome {
	if outcome == nil {
		return nil
	}
	clone := *outcome
	if outcome.Run != nil {
		runClone := *outcome.Run
		clone.Run = &runClone
	}
	if outcome.Queue != nil {
		queueClone := *outcome.Queue
		clone.Queue = &queueClone
	}
	if outcome.BudgetWarning != nil {
		warningClone := *outcome.BudgetWarning
		clone.BudgetWarning = &warningClone
	}
	return &clone
}

func defaultDispatchReason(reason string, fallback string) string {
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		return trimmed
	}
	return fallback
}

func cloneTaskDecompositionResponse(result *model.TaskDecompositionResponse) *model.TaskDecompositionResponse {
	if result == nil {
		return nil
	}
	clone := *result
	parentClone := result.ParentTask
	clone.ParentTask = parentClone
	if result.Subtasks != nil {
		clone.Subtasks = append([]model.TaskDTO(nil), result.Subtasks...)
	}
	return &clone
}
