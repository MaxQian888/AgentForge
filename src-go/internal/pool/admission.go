package pool

import (
	"context"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type AdmissionStatus string

const (
	AdmissionStatusStarted AdmissionStatus = "started"
	AdmissionStatusQueued  AdmissionStatus = "queued"
	AdmissionStatusBlocked AdmissionStatus = "blocked"
)

type QueueAdmissionInput struct {
	ProjectID uuid.UUID
	TaskID    uuid.UUID
	MemberID  uuid.UUID
	Runtime   string
	Provider  string
	Model     string
	RoleID    string
	BudgetUSD float64
	Reason    string
}

type QueueAdmissionWriter interface {
	QueueAgentAdmission(ctx context.Context, input QueueAdmissionInput) (*model.AgentPoolQueueEntry, error)
}

type AdmissionResult struct {
	Status AdmissionStatus
	Reason string
	Queue  *model.AgentPoolQueueEntry
}

type AdmissionController struct {
	pool        *Pool
	queueWriter QueueAdmissionWriter
}

func NewAdmissionController(pool *Pool, queueWriter QueueAdmissionWriter) *AdmissionController {
	return &AdmissionController{pool: pool, queueWriter: queueWriter}
}

func (c *AdmissionController) Decide(ctx context.Context, input QueueAdmissionInput) (*AdmissionResult, error) {
	if c.pool == nil || c.pool.Available() > 0 {
		return &AdmissionResult{Status: AdmissionStatusStarted}, nil
	}
	if c.queueWriter == nil {
		return &AdmissionResult{
			Status: AdmissionStatusBlocked,
			Reason: "agent pool is at capacity",
		}, nil
	}
	entry, err := c.queueWriter.QueueAgentAdmission(ctx, input)
	if err != nil {
		return nil, err
	}
	return &AdmissionResult{
		Status: AdmissionStatusQueued,
		Reason: input.Reason,
		Queue:  entry,
	}, nil
}
