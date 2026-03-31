package instruction

import (
	"context"
	"errors"
	"time"
)

type Target string

const (
	TargetLocal  Target = "local"
	TargetBridge Target = "bridge"
	TargetPlugin Target = "plugin"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

var (
	ErrInstructionNotRegistered = errors.New("instruction type is not registered")
	ErrInstructionNotFound      = errors.New("instruction not found")
	ErrNoPendingInstructions    = errors.New("no pending instructions")
	ErrNoRunnableInstruction    = errors.New("no runnable instructions")
	ErrDependencyFailed         = errors.New("instruction dependency failed")
)

type Request struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Payload      map[string]any    `json:"payload,omitempty"`
	Priority     int               `json:"priority,omitempty"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	PluginID     string            `json:"pluginId,omitempty"`
}

type Result struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Target      Target            `json:"target"`
	Status      Status            `json:"status"`
	Output      map[string]any    `json:"output,omitempty"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	StartedAt   time.Time         `json:"startedAt"`
	CompletedAt time.Time         `json:"completedAt"`
	Duration    time.Duration     `json:"duration"`
}

type PendingInstruction struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Target       Target            `json:"target"`
	Priority     int               `json:"priority"`
	Status       Status            `json:"status"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type Metrics struct {
	Successes     int
	Failures      int
	Cancelled     int
	Total         int
	TotalDuration time.Duration
	LastDuration  time.Duration
	LastError     string
	LastStatus    Status
}

type Definition struct {
	Target          Target
	DefaultTimeout  time.Duration
	DefaultPriority int
	Validator       Validator
	Handler         Handler
}

type Handler interface {
	Handle(ctx context.Context, req Request) (map[string]any, error)
}

type HandlerFunc func(ctx context.Context, req Request) (map[string]any, error)

func (fn HandlerFunc) Handle(ctx context.Context, req Request) (map[string]any, error) {
	return fn(ctx, req)
}

type Validator interface {
	Validate(req Request) error
}

type ValidatorFunc func(req Request) error

func (fn ValidatorFunc) Validate(req Request) error {
	return fn(req)
}
