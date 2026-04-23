package service_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	rolepkg "github.com/agentforge/server/internal/role"
	"github.com/agentforge/server/internal/service"
	"github.com/agentforge/server/internal/ws"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
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
	lastRoleID   string
	spawnCalls   int
	runs         []*model.AgentRun
}

func (m *mockDispatchRuntime) Spawn(_ context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	m.spawnCalls++
	m.lastTaskID = taskID
	m.lastMemberID = memberID
	m.lastRoleID = roleID
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

func (m *mockDispatchRuntime) GetByTask(_ context.Context, taskID uuid.UUID) ([]*model.AgentRun, error) {
	cloned := make([]*model.AgentRun, 0, len(m.runs))
	for _, run := range m.runs {
		if run == nil || run.TaskID != taskID {
			continue
		}
		copyRun := *run
		cloned = append(cloned, &copyRun)
	}
	return cloned, nil
}

type mockDispatchQueueWriter struct {
	entry *model.AgentPoolQueueEntry
	err   error
	last  service.QueueAgentAdmissionInput
}

type mockDispatchRoleStore struct {
	roles map[string]*rolepkg.Manifest
}

func (m *mockDispatchRoleStore) Get(id string) (*rolepkg.Manifest, error) {
	if role, ok := m.roles[id]; ok {
		return role, nil
	}
	return nil, os.ErrNotExist
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
		EntryID:             uuid.NewString(),
		ProjectID:           input.ProjectID.String(),
		TaskID:              input.TaskID.String(),
		MemberID:            input.MemberID.String(),
		Status:              model.AgentPoolQueueStatusQueued,
		Reason:              "agent pool is at capacity",
		Runtime:             input.Runtime,
		Provider:            input.Provider,
		Model:               input.Model,
		RoleID:              input.RoleID,
		BudgetUSD:           input.BudgetUSD,
		GuardrailType:       input.GuardrailType,
		GuardrailScope:      input.GuardrailScope,
		RecoveryDisposition: input.RecoveryDisposition,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
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

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil)

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

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil)

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

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil)

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

func TestTaskDispatchService_SpawnUsesMemberBoundRoleWhenExplicitRoleIsMissing(t *testing.T) {
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
			ID:          memberID,
			ProjectID:   projectID,
			Type:        model.MemberTypeAgent,
			IsActive:    true,
			AgentConfig: `{"roleId":"frontend-developer","runtime":"codex"}`,
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
	roleStore := &mockDispatchRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"frontend-developer": {
				Metadata: model.RoleMetadata{ID: "frontend-developer", Name: "Frontend Developer"},
			},
		},
	}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil).
		WithRoleStore(roleStore)

	result, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID: taskID,
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if result.Dispatch.RoleID != "frontend-developer" {
		t.Fatalf("dispatch roleId = %q, want frontend-developer", result.Dispatch.RoleID)
	}
	if runtime.lastRoleID != "frontend-developer" {
		t.Fatalf("runtime lastRoleID = %q, want frontend-developer", runtime.lastRoleID)
	}
}

func TestTaskDispatchService_SpawnBlocksWhenMemberBoundRoleIsStale(t *testing.T) {
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
			ID:          memberID,
			ProjectID:   projectID,
			Type:        model.MemberTypeAgent,
			IsActive:    true,
			AgentConfig: `{"roleId":"missing-role","runtime":"codex"}`,
		},
	}
	runtime := &mockDispatchRuntime{}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil).
		WithRoleStore(&mockDispatchRoleStore{roles: map[string]*rolepkg.Manifest{}})

	result, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID: taskID,
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusBlocked {
		t.Fatalf("dispatch result = %+v, want blocked", result.Dispatch)
	}
	if result.Dispatch.GuardrailType != model.DispatchGuardrailTypeTarget || result.Dispatch.GuardrailScope != "role" {
		t.Fatalf("guardrail = %+v, want target/role", result.Dispatch)
	}
	if runtime.spawnCalls != 0 {
		t.Fatalf("runtime spawn calls = %d, want 0", runtime.spawnCalls)
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

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil).WithQueueWriter(queueWriter)

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
	if result.Dispatch.Queue.GuardrailType != model.DispatchGuardrailTypePool || result.Dispatch.Queue.GuardrailScope != "project" {
		t.Fatalf("dispatch queue guardrail = %+v", result.Dispatch.Queue)
	}
	if result.Dispatch.Queue.RecoveryDisposition != model.QueueRecoveryDispositionPending {
		t.Fatalf("dispatch queue recoveryDisposition = %q", result.Dispatch.Queue.RecoveryDisposition)
	}
	if queueWriter.last.TaskID != taskID || queueWriter.last.MemberID != memberID {
		t.Fatalf("queue writer input = %+v", queueWriter.last)
	}
}

type mockDispatchAttemptRecorder struct {
	attempts []*model.DispatchAttempt
	err      error
}

func (m *mockDispatchAttemptRecorder) Create(_ context.Context, attempt *model.DispatchAttempt) error {
	if m.err != nil {
		return m.err
	}
	if attempt == nil {
		return nil
	}
	cloned := *attempt
	if attempt.MemberID != nil {
		memberID := *attempt.MemberID
		cloned.MemberID = &memberID
	}
	if attempt.QueuePriority != nil {
		priority := *attempt.QueuePriority
		cloned.QueuePriority = &priority
	}
	m.attempts = append(m.attempts, &cloned)
	return nil
}

func TestTaskDispatchService_RecordDispatchAttemptLogsRecorderFailure(t *testing.T) {
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
	recorder := &mockDispatchAttemptRecorder{err: errors.New("forced attempt write failure")}
	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil).WithAttemptRecorder(recorder)

	var buf bytes.Buffer
	oldOut := log.StandardLogger().Out
	log.SetOutput(&buf)
	defer log.SetOutput(oldOut)

	if _, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID:   taskID,
		Runtime:  "codex",
		Provider: "openai",
		Model:    "gpt-5-codex",
	}); err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if !strings.Contains(buf.String(), "dispatch attempt recording failed") {
		t.Fatalf("expected dispatch attempt failure log, got %s", buf.String())
	}
}

func TestTaskDispatchService_SpawnPreservesDispatchTupleAndQueueVerdict(t *testing.T) {
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
	runtime := &mockDispatchRuntime{err: service.ErrAgentPoolFull}
	queueWriter := &mockDispatchQueueWriter{}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil).WithQueueWriter(queueWriter)

	result, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID:    taskID,
		Runtime:   "codex",
		Provider:  "openai",
		Model:     "gpt-5-codex",
		RoleID:    "frontend-developer",
		BudgetUSD: 9,
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if result.Dispatch.Runtime != "codex" || result.Dispatch.Provider != "openai" || result.Dispatch.Model != "gpt-5-codex" || result.Dispatch.RoleID != "frontend-developer" {
		t.Fatalf("dispatch tuple = %+v", result.Dispatch)
	}
	if result.Dispatch.Queue == nil || result.Dispatch.Queue.GuardrailType != model.DispatchGuardrailTypePool {
		t.Fatalf("dispatch queue = %+v", result.Dispatch.Queue)
	}
}

func TestTaskDispatchService_RecordDispatchAttemptCapturesRichQueuedMetadata(t *testing.T) {
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
	runtime := &mockDispatchRuntime{err: service.ErrAgentPoolFull}
	queueWriter := &mockDispatchQueueWriter{}
	recorder := &mockDispatchAttemptRecorder{}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil).
		WithQueueWriter(queueWriter).
		WithAttemptRecorder(recorder)

	_, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID:    taskID,
		Runtime:   "codex",
		Provider:  "openai",
		Model:     "gpt-5-codex",
		RoleID:    "frontend-developer",
		BudgetUSD: 9,
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if len(recorder.attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(recorder.attempts))
	}
	attempt := recorder.attempts[0]
	if attempt.Runtime != "codex" || attempt.Provider != "openai" || attempt.Model != "gpt-5-codex" || attempt.RoleID != "frontend-developer" {
		t.Fatalf("attempt tuple = %+v", attempt)
	}
	if attempt.QueueEntryID == "" || attempt.QueuePriority == nil {
		t.Fatalf("attempt queue linkage = %+v", attempt)
	}
	if attempt.GuardrailType != model.DispatchGuardrailTypePool || attempt.GuardrailScope != "project" {
		t.Fatalf("attempt guardrail = %+v", attempt)
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

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil).WithBudgetChecker(checker)

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

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil).WithBudgetChecker(checker)

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
	pub, stop, events := subscribeProjectEvents(t, projectID.String())
	defer stop()

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), pub, nil, nil).WithBudgetChecker(checker)

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

func TestTaskDispatchService_SpawnBlocksWhenTaskAlreadyHasPausedRunInPreflight(t *testing.T) {
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
		runs: []*model.AgentRun{
			{ID: uuid.New(), TaskID: taskID, MemberID: memberID, Status: model.AgentRunStatusPaused},
		},
	}

	svc := service.NewTaskDispatchService(taskRepo, memberRepo, runtime, ws.NewHub(), nil, nil, nil)

	result, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID: taskID,
	})
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusBlocked {
		t.Fatalf("dispatch result = %+v, want blocked", result.Dispatch)
	}
	if result.Dispatch.GuardrailType != model.DispatchGuardrailTypeTask || result.Dispatch.GuardrailScope != "task" {
		t.Fatalf("guardrail = %+v, want task conflict", result.Dispatch)
	}
	if runtime.spawnCalls != 0 {
		t.Fatalf("runtime spawn calls = %d, want 0", runtime.spawnCalls)
	}
}
