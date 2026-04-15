package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeIMActionDispatcher struct {
	assignCalls int
	spawnCalls  int
	lastAssign  *model.AssignRequest
	lastSpawn   DispatchSpawnInput
	assignResp  *model.TaskDispatchResponse
	spawnResp   *model.TaskDispatchResponse
}

func (f *fakeIMActionDispatcher) Assign(ctx context.Context, taskID uuid.UUID, req *model.AssignRequest) (*model.TaskDispatchResponse, error) {
	f.assignCalls++
	f.lastAssign = req
	if f.assignResp != nil {
		return f.assignResp, nil
	}
	return nil, nil
}

func (f *fakeIMActionDispatcher) Spawn(ctx context.Context, input DispatchSpawnInput) (*model.TaskDispatchResponse, error) {
	f.spawnCalls++
	f.lastSpawn = input
	if f.spawnResp != nil {
		return f.spawnResp, nil
	}
	return nil, nil
}

type fakeIMActionDecomposer struct {
	calls int
	last  uuid.UUID
	resp  *model.TaskDecompositionResponse
	err   error
}

func (f *fakeIMActionDecomposer) Decompose(ctx context.Context, taskID uuid.UUID) (*model.TaskDecompositionResponse, error) {
	f.calls++
	f.last = taskID
	return f.resp, f.err
}

type fakeIMActionReviewer struct {
	getCalls            int
	approveCalls        int
	requestChangesCalls int
	routeFixCalls       int
	review              *model.Review
	approvedReview      *model.Review
	changesReview       *model.Review
	lastApproveNotes    string
	lastRequestComment  string
	lastActor           string
}

func (f *fakeIMActionReviewer) GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error) {
	f.getCalls++
	return f.review, nil
}

func (f *fakeIMActionReviewer) ApproveReview(ctx context.Context, id uuid.UUID, actor, comment string) (*model.Review, error) {
	f.approveCalls++
	f.lastActor = actor
	f.lastApproveNotes = comment
	return f.approvedReview, nil
}

func (f *fakeIMActionReviewer) RequestChangesReview(ctx context.Context, id uuid.UUID, actor, comment string) (*model.Review, error) {
	f.requestChangesCalls++
	f.lastActor = actor
	f.lastRequestComment = comment
	return f.changesReview, nil
}

func (f *fakeIMActionReviewer) RouteFixRequest(ctx context.Context, id uuid.UUID) error {
	f.routeFixCalls++
	return nil
}

type fakeIMActionTaskCreator struct {
	created *model.Task
}

func (f *fakeIMActionTaskCreator) Create(ctx context.Context, projectID uuid.UUID, req *model.CreateTaskRequest, reporterID *uuid.UUID) (*model.Task, error) {
	f.created = &model.Task{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       req.Title,
		Description: req.Description,
		Status:      model.TaskStatusInbox,
		Priority:    req.Priority,
	}
	return f.created, nil
}

type fakeIMActionTaskTransitioner struct {
	getTask         *model.Task
	getErr          error
	lastTaskID      uuid.UUID
	lastTargetState string
	transitionErr   error
}

func (f *fakeIMActionTaskTransitioner) GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	f.lastTaskID = id
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getTask, nil
}

func (f *fakeIMActionTaskTransitioner) TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error {
	f.lastTaskID = id
	f.lastTargetState = newStatus
	return f.transitionErr
}

func (f *fakeIMActionTaskTransitioner) Transition(ctx context.Context, id uuid.UUID, req *model.TransitionRequest) (*model.Task, error) {
	status := ""
	if req != nil {
		status = req.Status
	}
	if err := f.TransitionStatus(ctx, id, status); err != nil {
		return nil, err
	}
	if f.getTask == nil {
		return nil, nil
	}
	taskCopy := *f.getTask
	taskCopy.Status = status
	return &taskCopy, nil
}

type fakeIMActionBindingWriter struct {
	calls []*model.IMActionBinding
}

func (f *fakeIMActionBindingWriter) BindAction(ctx context.Context, binding *model.IMActionBinding) error {
	f.calls = append(f.calls, binding)
	return nil
}

type fakeIMActionProgressRecorder struct {
	calls []TaskActivityInput
}

func (f *fakeIMActionProgressRecorder) RecordActivity(ctx context.Context, taskID uuid.UUID, input TaskActivityInput) (*model.TaskProgressSnapshot, error) {
	f.calls = append(f.calls, input)
	return &model.TaskProgressSnapshot{TaskID: taskID}, nil
}

type fakeIMActionWorkflowEvaluator struct {
	calls []struct {
		from string
		to   string
		task *model.Task
	}
}

func (f *fakeIMActionWorkflowEvaluator) EvaluateTransition(ctx context.Context, task *model.Task, fromStatus, toStatus string) []TriggerResult {
	f.calls = append(f.calls, struct {
		from string
		to   string
		task *model.Task
	}{
		from: fromStatus,
		to:   toStatus,
		task: task,
	})
	return nil
}

type fakeIMActionWikiCreator struct {
	space *model.WikiSpace
	page  *model.WikiPage
}

func (f *fakeIMActionWikiCreator) GetSpaceByProjectID(ctx context.Context, projectID uuid.UUID) (*model.WikiSpace, error) {
	return f.space, nil
}

func (f *fakeIMActionWikiCreator) CreatePage(ctx context.Context, projectID uuid.UUID, spaceID uuid.UUID, title string, parentID *uuid.UUID, content string, createdBy *uuid.UUID) (*model.WikiPage, error) {
	f.page = &model.WikiPage{
		ID:      uuid.New(),
		SpaceID: spaceID,
		Title:   title,
		Content: content,
	}
	return f.page, nil
}

func TestBackendIMActionExecutor_AssignAgentUsesDispatchWorkflow(t *testing.T) {
	taskID := uuid.New()
	dispatcher := &fakeIMActionDispatcher{
		spawnResp: &model.TaskDispatchResponse{
			Task: model.TaskDTO{ID: taskID.String(), Title: "Bridge rollout"},
			Dispatch: model.DispatchOutcome{
				Status: model.DispatchStatusStarted,
				Run:    &model.AgentRunDTO{ID: uuid.NewString(), TaskID: taskID.String(), Status: "running"},
			},
		},
	}

	executor := NewBackendIMActionExecutor(dispatcher, nil, nil)
	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "assign-agent",
		EntityID:  taskID.String(),
		ChannelID: "C123",
		BridgeID:  "bridge-slack-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-1",
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if dispatcher.spawnCalls != 1 {
		t.Fatalf("spawnCalls = %d, want 1", dispatcher.spawnCalls)
	}
	if dispatcher.lastSpawn.TaskID != taskID {
		t.Fatalf("spawn task id = %s, want %s", dispatcher.lastSpawn.TaskID, taskID)
	}
	if resp.Status != model.IMActionStatusStarted {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Dispatch == nil || resp.Dispatch.Run == nil {
		t.Fatalf("dispatch = %+v", resp.Dispatch)
	}
	if resp.ReplyTarget == nil || resp.ReplyTarget.ThreadID != "thread-1" {
		t.Fatalf("replyTarget = %+v", resp.ReplyTarget)
	}
	if resp.Metadata[imMetadataDeliverySource] != imDeliverySourceActionResult {
		t.Fatalf("delivery source metadata = %+v", resp.Metadata)
	}
	if resp.Metadata[imMetadataBridgeBindingBridgeID] != "bridge-slack-1" {
		t.Fatalf("bridge binding metadata = %+v", resp.Metadata)
	}
	if resp.Metadata[imMetadataReplyTargetThreadID] != "thread-1" {
		t.Fatalf("reply target metadata = %+v", resp.Metadata)
	}
}

func TestBackendIMActionExecutor_RequestChangesBlocksStaleCompletedReview(t *testing.T) {
	reviewID := uuid.New()
	reviewer := &fakeIMActionReviewer{
		review: &model.Review{
			ID:             reviewID,
			Status:         model.ReviewStatusCompleted,
			Recommendation: model.ReviewRecommendationApprove,
		},
	}

	executor := NewBackendIMActionExecutor(nil, nil, reviewer)
	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "request-changes",
		EntityID:  reviewID.String(),
		ChannelID: "C123",
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if reviewer.requestChangesCalls != 0 {
		t.Fatalf("requestChangesCalls = %d, want 0", reviewer.requestChangesCalls)
	}
	if reviewer.routeFixCalls != 0 {
		t.Fatalf("routeFixCalls = %d, want 0", reviewer.routeFixCalls)
	}
	if resp.Status != model.IMActionStatusBlocked {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Success {
		t.Fatal("expected stale review action to be unsuccessful")
	}
}

func TestBackendIMActionExecutor_DecomposeReturnsCompletedOutcome(t *testing.T) {
	taskID := uuid.New()
	decomposer := &fakeIMActionDecomposer{
		resp: &model.TaskDecompositionResponse{
			ParentTask: model.TaskDTO{ID: taskID.String(), Title: "Bridge rollout"},
			Summary:    "Split into two subtasks",
			Subtasks: []model.TaskDTO{
				{ID: uuid.NewString(), Title: "Backend seam"},
				{ID: uuid.NewString(), Title: "Bridge replay"},
			},
		},
	}

	executor := NewBackendIMActionExecutor(nil, decomposer, nil)
	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "decompose",
		EntityID:  taskID.String(),
		ChannelID: "C123",
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if decomposer.calls != 1 || decomposer.last != taskID {
		t.Fatalf("decompose calls = %d last = %s", decomposer.calls, decomposer.last)
	}
	if resp.Status != model.IMActionStatusCompleted {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Decomposition == nil || len(resp.Decomposition.Subtasks) != 2 {
		t.Fatalf("decomposition = %+v", resp.Decomposition)
	}
}

func TestBackendIMActionExecutor_ApproveCompletesReview(t *testing.T) {
	reviewID := uuid.New()
	reviewer := &fakeIMActionReviewer{
		review: &model.Review{
			ID:     reviewID,
			Status: model.ReviewStatusPendingHuman,
		},
		approvedReview: &model.Review{
			ID:             reviewID,
			Status:         model.ReviewStatusCompleted,
			Recommendation: model.ReviewRecommendationApprove,
		},
	}

	executor := NewBackendIMActionExecutor(nil, nil, reviewer)
	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "approve",
		EntityID:  reviewID.String(),
		ChannelID: "C123",
		Metadata: map[string]string{
			"comment": "LGTM",
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if reviewer.approveCalls != 1 {
		t.Fatalf("approveCalls = %d", reviewer.approveCalls)
	}
	if reviewer.lastActor != "im-action" {
		t.Fatalf("actor = %q, want im-action", reviewer.lastActor)
	}
	if reviewer.lastApproveNotes != "LGTM" {
		t.Fatalf("approve comment = %q", reviewer.lastApproveNotes)
	}
	if resp.Status != model.IMActionStatusCompleted {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Review == nil || resp.Review.Recommendation != model.ReviewRecommendationApprove {
		t.Fatalf("review = %+v", resp.Review)
	}
}

func TestBackendIMActionExecutor_RequestChangesUsesTransitionMethod(t *testing.T) {
	reviewID := uuid.New()
	reviewer := &fakeIMActionReviewer{
		review: &model.Review{
			ID:             reviewID,
			Status:         model.ReviewStatusPendingHuman,
			Recommendation: model.ReviewRecommendationApprove,
		},
		changesReview: &model.Review{
			ID:             reviewID,
			Status:         model.ReviewStatusCompleted,
			Recommendation: model.ReviewRecommendationRequestChanges,
		},
	}

	executor := NewBackendIMActionExecutor(nil, nil, reviewer)
	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "request-changes",
		EntityID:  reviewID.String(),
		ChannelID: "C123",
		Metadata: map[string]string{
			"comment": "Please tighten validation",
			"actor":   "reviewer-42",
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if reviewer.requestChangesCalls != 1 {
		t.Fatalf("requestChangesCalls = %d, want 1", reviewer.requestChangesCalls)
	}
	if reviewer.lastActor != "reviewer-42" {
		t.Fatalf("actor = %q, want reviewer-42", reviewer.lastActor)
	}
	if reviewer.lastRequestComment != "Please tighten validation" {
		t.Fatalf("comment = %q, want request-changes comment", reviewer.lastRequestComment)
	}
	if reviewer.routeFixCalls != 1 {
		t.Fatalf("routeFixCalls = %d, want 1", reviewer.routeFixCalls)
	}
	if resp.Review == nil || resp.Review.Recommendation != model.ReviewRecommendationRequestChanges {
		t.Fatalf("review = %+v", resp.Review)
	}
}

func TestBackendIMActionExecutor_SaveAsDocCreatesWikiPage(t *testing.T) {
	projectID := uuid.New()
	wiki := &fakeIMActionWikiCreator{space: &model.WikiSpace{ID: uuid.New(), ProjectID: projectID}}
	executor := NewBackendIMActionExecutor(nil, nil, nil, wiki)

	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "save-as-doc",
		EntityID:  projectID.String(),
		ChannelID: "C123",
		Metadata: map[string]string{
			"title": "Incident Notes",
			"body":  "Captured from chat",
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if wiki.page == nil || wiki.page.Title != "Incident Notes" {
		t.Fatalf("page = %+v", wiki.page)
	}
	if resp.Status != model.IMActionStatusCompleted || resp.Metadata["href"] == "" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestBackendIMActionExecutor_CreateTaskCreatesBacklogTask(t *testing.T) {
	projectID := uuid.New()
	taskCreator := &fakeIMActionTaskCreator{}
	executor := NewBackendIMActionExecutor(nil, nil, nil, taskCreator)

	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "create-task",
		EntityID:  projectID.String(),
		ChannelID: "C123",
		Metadata: map[string]string{
			"title":    "Follow up",
			"body":     "Created from message",
			"priority": "high",
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if taskCreator.created == nil || taskCreator.created.Title != "Follow up" {
		t.Fatalf("created task = %+v", taskCreator.created)
	}
	if resp.Task == nil || resp.Task.Title != "Follow up" {
		t.Fatalf("response task = %+v", resp.Task)
	}
}

func TestBackendIMActionExecutor_CreateTaskFailsWhenWorkflowUnavailable(t *testing.T) {
	projectID := uuid.New()
	executor := NewBackendIMActionExecutor(nil, nil, nil)

	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "create-task",
		EntityID:  projectID.String(),
		ChannelID: "C123",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-1",
		},
		Metadata: map[string]string{
			"title": "Follow up",
			"body":  "Created from message",
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if resp == nil || resp.Success {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Status != model.IMActionStatusFailed {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.ReplyTarget == nil || resp.ReplyTarget.ThreadID != "thread-1" {
		t.Fatalf("reply target = %+v", resp.ReplyTarget)
	}
}

func TestBackendIMActionExecutor_TransitionTaskUsesCanonicalTransitionWorkflow(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	transitioner := &fakeIMActionTaskTransitioner{
		getTask: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Bridge rollout",
			Status:    model.TaskStatusInProgress,
			Priority:  "high",
		},
	}
	executor := NewBackendIMActionExecutor(nil, nil, nil, transitioner)

	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "feishu",
		Action:    "transition-task",
		EntityID:  taskID.String(),
		ChannelID: "chat-1",
		BridgeID:  "bridge-feishu-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:      "feishu",
			ChatID:        "chat-1",
			CallbackToken: "cb-token-1",
		},
		Metadata: map[string]string{
			"targetStatus": model.TaskStatusInProgress,
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if transitioner.lastTaskID != taskID {
		t.Fatalf("lastTaskID = %s, want %s", transitioner.lastTaskID, taskID)
	}
	if transitioner.lastTargetState != model.TaskStatusInProgress {
		t.Fatalf("lastTargetState = %q, want %q", transitioner.lastTargetState, model.TaskStatusInProgress)
	}
	if resp.Status != model.IMActionStatusCompleted {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Task == nil || resp.Task.ID != taskID.String() || resp.Task.Status != model.TaskStatusInProgress {
		t.Fatalf("task = %+v", resp.Task)
	}
	if resp.Metadata[imMetadataBridgeBindingTaskID] != taskID.String() {
		t.Fatalf("metadata = %+v", resp.Metadata)
	}
	if resp.Structured == nil || len(resp.Structured.Fields) == 0 {
		t.Fatalf("structured = %+v", resp.Structured)
	}
}

func TestBackendIMActionExecutor_TransitionTaskReturnsBlockedOutcomeForInvalidTransition(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	transitioner := &fakeIMActionTaskTransitioner{
		getTask: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Bridge rollout",
			Status:    model.TaskStatusDone,
			Priority:  "high",
		},
		transitionErr: errors.New("invalid transition from done to in_progress"),
	}
	executor := NewBackendIMActionExecutor(nil, nil, nil, transitioner)

	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "transition-task",
		EntityID:  taskID.String(),
		ChannelID: "C123",
		Metadata: map[string]string{
			"target_status": model.TaskStatusInProgress,
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if resp.Status != model.IMActionStatusBlocked {
		t.Fatalf("status = %q, want blocked", resp.Status)
	}
	if resp.Success {
		t.Fatal("expected blocked transition to be unsuccessful")
	}
	if resp.Task == nil || resp.Task.Status != model.TaskStatusDone {
		t.Fatalf("task = %+v, want original task context", resp.Task)
	}
}

func TestBackendIMActionExecutor_TransitionTaskStoresBindingAndEvaluatesWorkflow(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	transitioner := &fakeIMActionTaskTransitioner{
		getTask: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Bridge rollout",
			Status:    model.TaskStatusAssigned,
			Priority:  "high",
		},
	}
	binder := &fakeIMActionBindingWriter{}
	progress := &fakeIMActionProgressRecorder{}
	workflow := &fakeIMActionWorkflowEvaluator{}
	executor := NewBackendIMActionExecutor(nil, nil, nil, transitioner, binder, progress, workflow)

	resp, err := executor.Execute(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "transition-task",
		EntityID:  taskID.String(),
		ChannelID: "C123",
		BridgeID:  "bridge-slack-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-1",
		},
		Metadata: map[string]string{
			"target_status": model.TaskStatusInProgress,
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("response = %+v", resp)
	}
	if len(binder.calls) != 1 {
		t.Fatalf("binder calls = %d, want 1", len(binder.calls))
	}
	if binder.calls[0].TaskID != taskID.String() || binder.calls[0].ProjectID != projectID.String() {
		t.Fatalf("binding = %+v", binder.calls[0])
	}
	if len(progress.calls) != 1 || progress.calls[0].Source != model.TaskProgressSourceTaskTransition {
		t.Fatalf("progress calls = %+v", progress.calls)
	}
	if len(workflow.calls) != 1 {
		t.Fatalf("workflow calls = %d, want 1", len(workflow.calls))
	}
	if workflow.calls[0].from != model.TaskStatusAssigned || workflow.calls[0].to != model.TaskStatusInProgress {
		t.Fatalf("workflow call = %+v", workflow.calls[0])
	}
}

func TestIMService_HandleActionUsesExecutorOutcome(t *testing.T) {
	svc := NewIMService("", "slack")
	svc.SetActionExecutor(IMActionExecutorFunc(func(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error) {
		return &model.IMActionResponse{
			Result:      "Review approved",
			Success:     true,
			Status:      model.IMActionStatusCompleted,
			ReplyTarget: req.ReplyTarget,
			Metadata: map[string]string{
				"source": "block_actions",
			},
		}, nil
	}))

	resp, err := svc.HandleAction(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "approve",
		EntityID:  uuid.NewString(),
		ChannelID: "C123",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-1",
		},
	})
	if err != nil {
		t.Fatalf("HandleAction error: %v", err)
	}

	if resp.Status != model.IMActionStatusCompleted {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Metadata["source"] != "block_actions" {
		t.Fatalf("metadata = %+v", resp.Metadata)
	}
}

type fakeReactionRecorder struct {
	recorded []*model.IMReactionEvent
	err      error
}

func (r *fakeReactionRecorder) Record(ctx context.Context, event *model.IMReactionEvent) error {
	if r.err != nil {
		return r.err
	}
	r.recorded = append(r.recorded, event)
	return nil
}

func TestExecuteReact_RecordsEventAndReturnsCompleted(t *testing.T) {
	recorder := &fakeReactionRecorder{}
	exec := NewBackendIMActionExecutor(nil, nil, nil, recorder)

	req := &model.IMActionRequest{
		Platform:  "feishu",
		Action:    "react",
		EntityID:  "om_msg_1",
		ChannelID: "chat-1",
		UserID:    "ou_user",
		Metadata: map[string]string{
			"emoji":      "THUMBSUP",
			"event_type": "created",
		},
	}
	resp, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if resp.Status != model.IMActionStatusCompleted {
		t.Errorf("Status = %q, want completed", resp.Status)
	}
	if resp.Result != "" {
		t.Errorf("expected empty Result (so bridge does not post a reply); got %q", resp.Result)
	}
	if len(recorder.recorded) != 1 {
		t.Fatalf("expected 1 recorded event, got %d", len(recorder.recorded))
	}
	rec := recorder.recorded[0]
	if rec.MessageID != "om_msg_1" || rec.Emoji != "THUMBSUP" || rec.EventType != "created" {
		t.Errorf("recorded event = %+v", rec)
	}
}

func TestExecuteReact_WithoutRecorderReturnsBlocked(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "react",
		EntityID: "om_msg_1",
		Metadata: map[string]string{"emoji": "OK", "event_type": "created"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
}
