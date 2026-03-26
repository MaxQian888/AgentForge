package service

import (
	"context"
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
	getCalls         int
	approveCalls     int
	completeCalls    int
	routeFixCalls    int
	review           *model.Review
	approvedReview   *model.Review
	completedReview  *model.Review
	lastCompleteReq  *model.CompleteReviewRequest
	lastApproveNotes string
}

func (f *fakeIMActionReviewer) GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error) {
	f.getCalls++
	return f.review, nil
}

func (f *fakeIMActionReviewer) Approve(ctx context.Context, id uuid.UUID, comment string) (*model.Review, error) {
	f.approveCalls++
	f.lastApproveNotes = comment
	return f.approvedReview, nil
}

func (f *fakeIMActionReviewer) Complete(ctx context.Context, id uuid.UUID, req *model.CompleteReviewRequest) (*model.Review, error) {
	f.completeCalls++
	f.lastCompleteReq = req
	return f.completedReview, nil
}

func (f *fakeIMActionReviewer) RouteFixRequest(ctx context.Context, id uuid.UUID) error {
	f.routeFixCalls++
	return nil
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

	if reviewer.completeCalls != 0 {
		t.Fatalf("completeCalls = %d, want 0", reviewer.completeCalls)
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
			Status: model.ReviewStatusInProgress,
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
