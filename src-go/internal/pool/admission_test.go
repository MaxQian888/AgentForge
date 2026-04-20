package pool_test

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/pool"
	"github.com/google/uuid"
)

type queueWriterStub struct {
	entry *model.AgentPoolQueueEntry
}

func (s *queueWriterStub) QueueAgentAdmission(_ context.Context, input pool.QueueAdmissionInput) (*model.AgentPoolQueueEntry, error) {
	if s.entry != nil {
		return s.entry, nil
	}
	return &model.AgentPoolQueueEntry{
		EntryID:   uuid.NewString(),
		ProjectID: input.ProjectID.String(),
		TaskID:    input.TaskID.String(),
		MemberID:  input.MemberID.String(),
		Status:    model.AgentPoolQueueStatusQueued,
		Reason:    input.Reason,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func TestAdmissionController_DecidesStartedWhenSlotAvailable(t *testing.T) {
	controller := pool.NewAdmissionController(pool.NewPool(1), nil)

	result, err := controller.Decide(context.Background(), pool.QueueAdmissionInput{
		ProjectID: uuid.New(),
		TaskID:    uuid.New(),
		MemberID:  uuid.New(),
		Reason:    "agent pool is at capacity",
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if result.Status != pool.AdmissionStatusStarted {
		t.Fatalf("result.Status = %q, want started", result.Status)
	}
}

func TestAdmissionController_QueuesWhenFullAndQueueWriterPresent(t *testing.T) {
	p := pool.NewPool(1)
	if err := p.Acquire("run-1", "task-1", "member-1"); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	controller := pool.NewAdmissionController(p, &queueWriterStub{})

	result, err := controller.Decide(context.Background(), pool.QueueAdmissionInput{
		ProjectID: uuid.New(),
		TaskID:    uuid.New(),
		MemberID:  uuid.New(),
		Reason:    "agent pool is at capacity",
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if result.Status != pool.AdmissionStatusQueued || result.Queue == nil {
		t.Fatalf("result = %+v, want queued with queue entry", result)
	}
}

func TestAdmissionController_BlocksWhenFullWithoutQueueWriter(t *testing.T) {
	p := pool.NewPool(1)
	if err := p.Acquire("run-1", "task-1", "member-1"); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	controller := pool.NewAdmissionController(p, nil)

	result, err := controller.Decide(context.Background(), pool.QueueAdmissionInput{
		ProjectID: uuid.New(),
		TaskID:    uuid.New(),
		MemberID:  uuid.New(),
		Reason:    "agent pool is at capacity",
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if result.Status != pool.AdmissionStatusBlocked {
		t.Fatalf("result.Status = %q, want blocked", result.Status)
	}
}
