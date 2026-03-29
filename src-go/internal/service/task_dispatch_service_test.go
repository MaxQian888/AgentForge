package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type mockDispatchTaskRepo struct {
	task             *model.Task
	updatedAssignee  uuid.UUID
	updatedType      string
	updateCalls      int
	transitionCalls  int
	transitionStatus string
}

func (m *mockDispatchTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	if m.task == nil || m.task.ID != id {
		return nil, service.ErrAgentTaskNotFound
	}
	cloned := *m.task
	return &cloned, nil
}

func (m *mockDispatchTaskRepo) UpdateAssignee(_ context.Context, _ uuid.UUID, assigneeID uuid.UUID, assigneeType string) error {
	m.updateCalls++
	m.updatedAssignee = assigneeID
	m.updatedType = assigneeType
	if m.task != nil {
		m.task.AssigneeID = &assigneeID
		m.task.AssigneeType = assigneeType
	}
	return nil
}

func (m *mockDispatchTaskRepo) TransitionStatus(_ context.Context, _ uuid.UUID, newStatus string) error {
	m.transitionCalls++
	m.transitionStatus = newStatus
	if m.task != nil {
		m.task.Status = newStatus
	}
	return nil
}

type mockDispatchMemberRepo struct {
	member *model.Member
}

func (m *mockDispatchMemberRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Member, error) {
	if m.member == nil || m.member.ID != id {
		return nil, service.ErrDispatchMemberNotFound
	}
	cloned := *m.member
	return &cloned, nil
}

type mockDispatchRuntime struct {
	run          *model.AgentRun
	err          error
	lastTaskID   uuid.UUID
	lastMemberID uuid.UUID
	spawnCalls   int
}

func (m *mockDispatchRuntime) Spawn(_ context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	m.spawnCalls++
	m.lastTaskID = taskID
	m.lastMemberID = memberID
	if m.err != nil {
		return nil, m.err
	}
	if m.run != nil {
		return m.run, nil
	}
	return &model.AgentRun{
		ID:       uuid.New(),
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}, nil
}

type mockDispatchQueueWriter struct {
	entry *model.AgentPoolQueueEntry
	err   error
	last  service.QueueAgentAdmissionInput
}

func (m *mockDispatchQueueWriter) QueueAgentAdmission(_ context.Context, input service.QueueAgentAdmissionInput) (*model.AgentPoolQueueEntry, error) {
	m.last = input
	if m.err != nil {
		return nil, m.err
	}
	if m.entry != nil {
		return m.entry, nil
	}
	return &model.AgentPoolQueueEntry{
		EntryID:   uuid.NewString(),
		ProjectID: input.ProjectID.String(),
		TaskID:    input.TaskID.String(),
		MemberID:  input.MemberID.String(),
		Status:    model.AgentPoolQueueStatusQueued,
		Reason:    "agent pool is at capacity",
		Runtime:   input.Runtime,
		Provider:  input.Provider,
		Model:     input.Model,
		RoleID:    input.RoleID,
		BudgetUSD: input.BudgetUSD,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

type mockDispatchBudgetChecker struct {
	result        *service.BudgetCheckResult
	err           error
	lastProjectID uuid.UUID
	lastSprintID  *uuid.UUID
	lastRequested float64
	callCount     int
}

func (m *mockDispatchBudgetChecker) CheckBudget(_ context.Context, projectID uuid.UUID, sprintID *uuid.UUID, requestedUsd float64) (*service.BudgetCheckResult, error) {
	m.callCount++
	m.lastProjectID = projectID
	m.lastSprintID = sprintID
	m.lastRequested = requestedUsd
	if m.err != nil {
		return nil, m.err
	}
	if m.result == nil {
		return &service.BudgetCheckResult{Allowed: true}, nil
	}
	cloned := *m.result
	return &cloned, nil
}

func TestTaskDispatchService_AssignAgentStartsRuntime(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	memberID := uuid.New()
	taskRepo := &mockDispatchTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Dispatch task",
			Status:    model.TaskStatusTriaged,
		},
	}
	memberRepo := &mockDispatchMemberRepo{
		member: &model.Member{
			ID:        memberID,
			ProjectID: projectID,
			Type:      model.MemberTypeAgent,
			IsActive:  true,
		},
	}
	runtime := &mockDispatchRuntime{
		run: &model.AgentRun{
			ID:       uuid.New(),
			TaskID:   taskID,
			MemberID: memberID,
			Status:   model.AgentRunStatusRunning,
		},
	}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil)

	result, err := svc.Assign(context.Background(), taskID, &model.AssignRequest{
		AssigneeID:   memberID.String(),
		AssigneeType: model.MemberTypeAgent,
	})
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusStarted {
		t.Fatalf("dispatch result = %+v, want started", result.Dispatch)
	}
	if result.Dispatch.Run == nil || result.Dispatch.Run.MemberID != memberID.String() {
		t.Fatalf("dispatch run = %+v", result.Dispatch.Run)
	}
	if taskRepo.updateCalls != 1 || taskRepo.updatedAssignee != memberID {
		t.Fatalf("assignment persisted = %d/%s", taskRepo.updateCalls, taskRepo.updatedAssignee)
	}
	if taskRepo.transitionStatus != model.TaskStatusAssigned {
		t.Fatalf("transition status = %q, want %q", taskRepo.transitionStatus, model.TaskStatusAssigned)
	}
	if runtime.lastTaskID != taskID || runtime.lastMemberID != memberID {
		t.Fatalf("runtime spawn called with %s/%s", runtime.lastTaskID, runtime.lastMemberID)
	}
}

func TestTaskDispatchService_AssignAgentReturnsBlockedWhenRunAlreadyActive(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	memberID := uuid.New()
	taskRepo := &mockDispatchTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Dispatch task",
			Status:    model.TaskStatusTriaged,
		},
	}
	memberRepo := &mockDispatchMemberRepo{
		member: &model.Member{
			ID:        memberID,
			ProjectID: projectID,
			Type:      model.MemberTypeAgent,
			IsActive:  true,
		},
	}
	runtime := &mockDispatchRuntime{err: service.ErrAgentAlreadyRunning}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil)

	result, err := svc.Assign(context.Background(), taskID, &model.AssignRequest{
		AssigneeID:   memberID.String(),
		AssigneeType: model.MemberTypeAgent,
	})
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusBlocked {
		t.Fatalf("dispatch result = %+v, want blocked", result.Dispatch)
	}
	if result.Dispatch.Run != nil {
		t.Fatalf("dispatch run = %+v, want nil", result.Dispatch.Run)
	}
	if taskRepo.updateCalls != 1 || taskRepo.updatedAssignee != memberID {
		t.Fatalf("assignment persisted = %d/%s", taskRepo.updateCalls, taskRepo.updatedAssignee)
	}
}

func TestTaskDispatchService_SpawnUsesAssignedAgentWhenMemberIDMissing(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	memberID := uuid.New()
	taskRepo := &mockDispatchTaskRepo{
		task: &model.Task{
			ID:           taskID,
			ProjectID:    projectID,
			Title:        "Dispatch task",
			Status:       model.TaskStatusAssigned,
			AssigneeID:   &memberID,
			AssigneeType: model.MemberTypeAgent,
		},
	}
	memberRepo := &mockDispatchMemberRepo{
		member: &model.Member{
			ID:        memberID,
			ProjectID: projectID,
			Type:      model.MemberTypeAgent,
			IsActive:  true,
		},
	}
	runtime := &mockDispatchRuntime{
		run: &model.AgentRun{
			ID:       uuid.New(),
			TaskID:   taskID,
			MemberID: memberID,
			Status:   model.AgentRunStatusRunning,
		},
	}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil)

	result, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID: taskID,
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusStarted {
		t.Fatalf("dispatch result = %+v, want started", result.Dispatch)
	}
	if runtime.lastMemberID != memberID {
		t.Fatalf("runtime member = %s, want %s", runtime.lastMemberID, memberID)
	}
}

func TestTaskDispatchService_AssignAgentQueuesWhenPoolIsFull(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	memberID := uuid.New()
	taskRepo := &mockDispatchTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Dispatch task",
			Status:    model.TaskStatusTriaged,
		},
	}
	memberRepo := &mockDispatchMemberRepo{
		member: &model.Member{
			ID:        memberID,
			ProjectID: projectID,
			Type:      model.MemberTypeAgent,
			IsActive:  true,
		},
	}
	runtime := &mockDispatchRuntime{err: service.ErrAgentPoolFull}
	queueWriter := &mockDispatchQueueWriter{}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil).WithQueueWriter(queueWriter)

	result, err := svc.Assign(context.Background(), taskID, &model.AssignRequest{
		AssigneeID:   memberID.String(),
		AssigneeType: model.MemberTypeAgent,
	})
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusQueued {
		t.Fatalf("dispatch result = %+v, want queued", result.Dispatch)
	}
	if result.Dispatch.Queue == nil {
		t.Fatalf("dispatch queue payload = nil, want queue entry")
	}
	if result.Dispatch.Queue.TaskID != taskID.String() || result.Dispatch.Queue.MemberID != memberID.String() {
		t.Fatalf("dispatch queue payload = %+v", result.Dispatch.Queue)
	}
	if queueWriter.last.TaskID != taskID || queueWriter.last.MemberID != memberID {
		t.Fatalf("queue writer input = %+v", queueWriter.last)
	}
}

func TestTaskDispatchService_AssignAgentBlocksWhenSprintBudgetExceeded(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	sprintID := uuid.New()
	memberID := uuid.New()
	taskRepo := &mockDispatchTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			SprintID:  &sprintID,
			Title:     "Dispatch task",
			Status:    model.TaskStatusTriaged,
		},
	}
	memberRepo := &mockDispatchMemberRepo{
		member: &model.Member{
			ID:        memberID,
			ProjectID: projectID,
			Type:      model.MemberTypeAgent,
			IsActive:  true,
		},
	}
	runtime := &mockDispatchRuntime{}
	checker := &mockDispatchBudgetChecker{
		result: &service.BudgetCheckResult{
			Allowed: false,
			Reason:  "sprint budget exceeded",
		},
	}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil).WithBudgetChecker(checker)

	result, err := svc.Assign(context.Background(), taskID, &model.AssignRequest{
		AssigneeID:   memberID.String(),
		AssigneeType: model.MemberTypeAgent,
	})
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusBlocked {
		t.Fatalf("dispatch result = %+v, want blocked", result.Dispatch)
	}
	if result.Dispatch.GuardrailType != "budget" {
		t.Fatalf("guardrailType = %q, want budget", result.Dispatch.GuardrailType)
	}
	if result.Dispatch.GuardrailScope != "sprint" {
		t.Fatalf("guardrailScope = %q, want sprint", result.Dispatch.GuardrailScope)
	}
	if runtime.spawnCalls != 0 {
		t.Fatalf("runtime spawn calls = %d, want 0", runtime.spawnCalls)
	}
	if checker.callCount != 1 || checker.lastProjectID != projectID || checker.lastSprintID == nil || *checker.lastSprintID != sprintID {
		t.Fatalf("budget checker call = %#v", checker)
	}
}

func TestTaskDispatchService_SpawnBlocksWhenProjectBudgetExceeded(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	memberID := uuid.New()
	taskRepo := &mockDispatchTaskRepo{
		task: &model.Task{
			ID:           taskID,
			ProjectID:    projectID,
			Title:        "Dispatch task",
			Status:       model.TaskStatusAssigned,
			AssigneeID:   &memberID,
			AssigneeType: model.MemberTypeAgent,
		},
	}
	memberRepo := &mockDispatchMemberRepo{
		member: &model.Member{
			ID:        memberID,
			ProjectID: projectID,
			Type:      model.MemberTypeAgent,
			IsActive:  true,
		},
	}
	runtime := &mockDispatchRuntime{}
	checker := &mockDispatchBudgetChecker{
		result: &service.BudgetCheckResult{
			Allowed: false,
			Reason:  "project budget exceeded",
		},
	}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil).WithBudgetChecker(checker)

	result, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID:    taskID,
		BudgetUSD: 7.5,
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusBlocked {
		t.Fatalf("dispatch result = %+v, want blocked", result.Dispatch)
	}
	if result.Dispatch.GuardrailType != "budget" {
		t.Fatalf("guardrailType = %q, want budget", result.Dispatch.GuardrailType)
	}
	if result.Dispatch.GuardrailScope != "project" {
		t.Fatalf("guardrailScope = %q, want project", result.Dispatch.GuardrailScope)
	}
	if runtime.spawnCalls != 0 {
		t.Fatalf("runtime spawn calls = %d, want 0", runtime.spawnCalls)
	}
	if checker.lastRequested != 7.5 {
		t.Fatalf("budget checker requestedUsd = %v, want 7.5", checker.lastRequested)
	}
}

func TestTaskDispatchService_AssignAgentReturnsBudgetWarningAndBroadcastsEvent(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	sprintID := uuid.New()
	memberID := uuid.New()
	taskRepo := &mockDispatchTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			SprintID:  &sprintID,
			Title:     "Dispatch task",
			Status:    model.TaskStatusTriaged,
		},
	}
	memberRepo := &mockDispatchMemberRepo{
		member: &model.Member{
			ID:        memberID,
			ProjectID: projectID,
			Type:      model.MemberTypeAgent,
			IsActive:  true,
		},
	}
	runtime := &mockDispatchRuntime{
		run: &model.AgentRun{
			ID:       uuid.New(),
			TaskID:   taskID,
			MemberID: memberID,
			Status:   model.AgentRunStatusRunning,
		},
	}
	checker := &mockDispatchBudgetChecker{
		result: &service.BudgetCheckResult{
			Allowed:        true,
			Warning:        true,
			WarningMessage: "sprint budget warning",
		},
	}
	hub := ws.NewHub()
	stop, events := subscribeProjectEvents(t, hub, projectID.String())
	defer stop()
	waitForHubClient(t, hub)

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, hub, nil, nil).WithBudgetChecker(checker)

	result, err := svc.Assign(context.Background(), taskID, &model.AssignRequest{
		AssigneeID:   memberID.String(),
		AssigneeType: model.MemberTypeAgent,
	})
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusStarted {
		t.Fatalf("dispatch result = %+v, want started", result.Dispatch)
	}
	if result.Dispatch.BudgetWarning == nil {
		t.Fatal("expected budget warning on dispatch outcome")
	}
	if result.Dispatch.BudgetWarning.Scope != "sprint" {
		t.Fatalf("budget warning scope = %q, want sprint", result.Dispatch.BudgetWarning.Scope)
	}
	if result.Dispatch.BudgetWarning.Message != "sprint budget warning" {
		t.Fatalf("budget warning message = %q", result.Dispatch.BudgetWarning.Message)
	}

	warning := waitForEventType(t, events, ws.EventBudgetWarning)
	if warning.Type != ws.EventBudgetWarning {
		t.Fatalf("warning event type = %q, want %q", warning.Type, ws.EventBudgetWarning)
	}
}

func waitForHubClient(t *testing.T, hub *ws.Hub) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ClientCount() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for websocket client registration")
}
