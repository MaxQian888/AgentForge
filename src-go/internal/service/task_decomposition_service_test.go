package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeTaskDecompositionRepo struct {
	task          *model.Task
	getErr        error
	hasChildren   bool
	hasChildrenErr error
	createErr     error
	createInputs  []model.TaskChildInput
	createResult  []*model.Task
}

func (f *fakeTaskDecompositionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.task == nil || f.task.ID != id {
		return nil, errors.New("not found")
	}
	return f.task, nil
}

func (f *fakeTaskDecompositionRepo) HasChildren(ctx context.Context, parentID uuid.UUID) (bool, error) {
	if f.hasChildrenErr != nil {
		return false, f.hasChildrenErr
	}
	return f.hasChildren, nil
}

func (f *fakeTaskDecompositionRepo) CreateChildren(ctx context.Context, inputs []model.TaskChildInput) ([]*model.Task, error) {
	f.createInputs = append([]model.TaskChildInput(nil), inputs...)
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResult != nil {
		return f.createResult, nil
	}
	return nil, nil
}

type fakeTaskDecompositionBridge struct {
	resp    *BridgeDecomposeResponse
	err     error
	lastReq *BridgeDecomposeRequest
}

func (f *fakeTaskDecompositionBridge) DecomposeTask(ctx context.Context, req BridgeDecomposeRequest) (*BridgeDecomposeResponse, error) {
	reqCopy := req
	f.lastReq = &reqCopy
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func TestTaskDecompositionService_DecomposeSuccess(t *testing.T) {
	parentID := uuid.New()
	projectID := uuid.New()
	sprintID := uuid.New()
	parent := &model.Task{
		ID:        parentID,
		ProjectID: projectID,
		SprintID:  &sprintID,
		Title:     "Bridge task decomposition",
		Description: "Implement decomposition across the bridge, Go API, and IM commands.",
		Priority:  "high",
		Status:    model.TaskStatusTriaged,
	}
	child1 := &model.Task{ID: uuid.New(), ProjectID: projectID, ParentID: &parentID, SprintID: &sprintID, Title: "Bridge route", Description: "Add /bridge/decompose", Priority: "high", Status: model.TaskStatusInbox}
	child2 := &model.Task{ID: uuid.New(), ProjectID: projectID, ParentID: &parentID, SprintID: &sprintID, Title: "IM command", Description: "Add /task decompose", Priority: "medium", Status: model.TaskStatusInbox}

	repo := &fakeTaskDecompositionRepo{
		task:         parent,
		createResult: []*model.Task{child1, child2},
	}
	bridge := &fakeTaskDecompositionBridge{
		resp: &BridgeDecomposeResponse{
			Summary: "Break the work into bridge and integration steps.",
			Subtasks: []BridgeDecomposeSubtask{
				{Title: "Bridge route", Description: "Add /bridge/decompose", Priority: "invalid"},
				{Title: "IM command", Description: "Add /task decompose", Priority: "medium"},
			},
		},
	}

	svc := NewTaskDecompositionService(repo, bridge)

	result, err := svc.Decompose(context.Background(), parentID)
	if err != nil {
		t.Fatalf("Decompose() error = %v", err)
	}

	if bridge.lastReq == nil || bridge.lastReq.TaskID != parentID.String() {
		t.Fatalf("expected bridge request for parent task, got %+v", bridge.lastReq)
	}
	if len(repo.createInputs) != 2 {
		t.Fatalf("expected 2 child inputs, got %d", len(repo.createInputs))
	}
	if repo.createInputs[0].ParentID != parentID {
		t.Errorf("first child parent id = %s, want %s", repo.createInputs[0].ParentID, parentID)
	}
	if repo.createInputs[0].Priority != parent.Priority {
		t.Errorf("invalid priority should fall back to parent priority %q, got %q", parent.Priority, repo.createInputs[0].Priority)
	}
	if result.ParentTask.ID != parentID.String() {
		t.Errorf("parent task id = %q, want %q", result.ParentTask.ID, parentID.String())
	}
	if len(result.Subtasks) != 2 {
		t.Fatalf("expected 2 subtasks in response, got %d", len(result.Subtasks))
	}
}

func TestTaskDecompositionService_DecomposeMissingTask(t *testing.T) {
	svc := NewTaskDecompositionService(&fakeTaskDecompositionRepo{}, &fakeTaskDecompositionBridge{})

	_, err := svc.Decompose(context.Background(), uuid.New())
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestTaskDecompositionService_RejectsExistingChildren(t *testing.T) {
	parent := &model.Task{ID: uuid.New(), ProjectID: uuid.New(), Title: "Existing decomposition", Description: "Already split", Priority: "medium"}
	repo := &fakeTaskDecompositionRepo{task: parent, hasChildren: true}
	bridge := &fakeTaskDecompositionBridge{}

	svc := NewTaskDecompositionService(repo, bridge)

	_, err := svc.Decompose(context.Background(), parent.ID)
	if !errors.Is(err, ErrTaskAlreadyDecomposed) {
		t.Fatalf("expected ErrTaskAlreadyDecomposed, got %v", err)
	}
	if bridge.lastReq != nil {
		t.Fatalf("bridge should not be called when task already has children")
	}
}

func TestTaskDecompositionService_RejectsInvalidBridgeOutput(t *testing.T) {
	parent := &model.Task{ID: uuid.New(), ProjectID: uuid.New(), Title: "Invalid output", Description: "Bridge returned nothing", Priority: "medium"}
	repo := &fakeTaskDecompositionRepo{task: parent}
	bridge := &fakeTaskDecompositionBridge{
		resp: &BridgeDecomposeResponse{
			Summary: "",
		},
	}

	svc := NewTaskDecompositionService(repo, bridge)

	_, err := svc.Decompose(context.Background(), parent.ID)
	if !errors.Is(err, ErrInvalidTaskDecomposition) {
		t.Fatalf("expected ErrInvalidTaskDecomposition, got %v", err)
	}
	if len(repo.createInputs) != 0 {
		t.Fatalf("expected no child creation on invalid bridge output")
	}
}

func TestTaskDecompositionService_DoesNotPersistPartialChildrenOnCreateFailure(t *testing.T) {
	parent := &model.Task{ID: uuid.New(), ProjectID: uuid.New(), Title: "Create failure", Description: "Persistence should fail atomically", Priority: "low"}
	repo := &fakeTaskDecompositionRepo{
		task:      parent,
		createErr: errors.New("transaction failed"),
	}
	bridge := &fakeTaskDecompositionBridge{
		resp: &BridgeDecomposeResponse{
			Summary: "A single child should be rejected atomically.",
			Subtasks: []BridgeDecomposeSubtask{
				{Title: "Child", Description: "Only child", Priority: "low"},
			},
		},
	}

	svc := NewTaskDecompositionService(repo, bridge)

	_, err := svc.Decompose(context.Background(), parent.ID)
	if err == nil {
		t.Fatal("expected create failure")
	}
	if len(repo.createInputs) != 1 {
		t.Fatalf("expected 1 attempted child input, got %d", len(repo.createInputs))
	}
	if repo.createResult != nil {
		t.Fatalf("expected no persisted child results on failure")
	}
}
