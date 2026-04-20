package instruction

import (
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	memorypkg "github.com/agentforge/server/internal/memory"
)

const defaultHistoryLimit = 256

type InstructionRouter struct {
	mu          sync.Mutex
	definitions map[string]Definition
	pending     instructionPriorityQueue
	pendingByID map[string]*instructionQueueItem
	active      map[string]context.CancelFunc
	completed   map[string]Result
	history     []Result
	metrics     map[string]Metrics
	historyCap  int
	shortTerm   shortTermMemoryStore
	now         func() time.Time
	sequence    uint64
}

type shortTermMemoryStore interface {
	Store(input memorypkg.StoreInput) (memorypkg.Entry, error)
}

func NewRouter() *InstructionRouter {
	return &InstructionRouter{
		definitions: make(map[string]Definition),
		pending:     make(instructionPriorityQueue, 0),
		pendingByID: make(map[string]*instructionQueueItem),
		active:      make(map[string]context.CancelFunc),
		completed:   make(map[string]Result),
		history:     make([]Result, 0, defaultHistoryLimit),
		metrics:     make(map[string]Metrics),
		historyCap:  defaultHistoryLimit,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (r *InstructionRouter) WithShortTermMemory(store shortTermMemoryStore) *InstructionRouter {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shortTerm = store
	return r
}

func (r *InstructionRouter) Register(instructionType string, definition Definition) error {
	name := strings.TrimSpace(instructionType)
	if name == "" {
		return fmt.Errorf("instruction type is required")
	}
	if definition.Target == "" {
		return fmt.Errorf("instruction %s target is required", name)
	}
	if definition.Handler == nil {
		return fmt.Errorf("instruction %s handler is required", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions[name] = definition
	return nil
}

func (r *InstructionRouter) Enqueue(req Request) error {
	req, definition, err := r.prepareRequest(req)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.pendingByID[req.ID]; exists {
		return fmt.Errorf("instruction %s already queued", req.ID)
	}
	if _, exists := r.active[req.ID]; exists {
		return fmt.Errorf("instruction %s is already running", req.ID)
	}
	if _, exists := r.completed[req.ID]; exists {
		return fmt.Errorf("instruction %s already completed", req.ID)
	}

	item := &instructionQueueItem{
		request:    req,
		definition: definition,
		sequence:   atomic.AddUint64(&r.sequence, 1),
	}
	heap.Push(&r.pending, item)
	r.pendingByID[req.ID] = item
	return nil
}

func (r *InstructionRouter) Execute(ctx context.Context, req Request) (Result, error) {
	req, definition, err := r.prepareRequest(req)
	if err != nil {
		return r.finishWithoutExecution(req, definition, StatusFailed, err), err
	}

	if depErr := r.evaluateDependencies(req.Dependencies); depErr != nil {
		return r.finishWithoutExecution(req, definition, StatusFailed, depErr), depErr
	}

	return r.executePrepared(ctx, req, definition)
}

func (r *InstructionRouter) ProcessNext(ctx context.Context) (Result, error) {
	r.mu.Lock()
	if len(r.pending) == 0 {
		r.mu.Unlock()
		return Result{}, ErrNoPendingInstructions
	}

	skipped := make([]*instructionQueueItem, 0, len(r.pending))
	var selected *instructionQueueItem
	var dependencyFailure Result
	var dependencyErr error

	for len(r.pending) > 0 {
		item := heap.Pop(&r.pending).(*instructionQueueItem)
		delete(r.pendingByID, item.request.ID)

		waiting, depErr := r.evaluateDependenciesLocked(item.request.Dependencies)
		if depErr != nil {
			now := r.now()
			dependencyFailure = r.recordResultLocked(item.request, item.definition, Result{
				ID:          item.request.ID,
				Type:        item.request.Type,
				Target:      item.definition.Target,
				Status:      StatusFailed,
				Error:       depErr.Error(),
				StartedAt:   now,
				CompletedAt: now,
				Duration:    0,
			})
			dependencyErr = depErr
			break
		}
		if waiting {
			skipped = append(skipped, item)
			continue
		}
		selected = item
		break
	}

	for _, item := range skipped {
		heap.Push(&r.pending, item)
		r.pendingByID[item.request.ID] = item
	}
	r.mu.Unlock()

	if dependencyErr != nil {
		return dependencyFailure, dependencyErr
	}
	if selected == nil {
		return Result{}, ErrNoRunnableInstruction
	}

	return r.executePrepared(ctx, selected.request, selected.definition)
}

func (r *InstructionRouter) Pending() []PendingInstruction {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]PendingInstruction, 0, len(r.pending))
	for _, item := range r.pending {
		items = append(items, PendingInstruction{
			ID:           item.request.ID,
			Type:         item.request.Type,
			Target:       item.definition.Target,
			Priority:     item.request.Priority,
			Status:       StatusQueued,
			Dependencies: append([]string(nil), item.request.Dependencies...),
			Metadata:     cloneStringMap(item.request.Metadata),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			return items[i].ID < items[j].ID
		}
		return items[i].Priority > items[j].Priority
	})
	return items
}

func (r *InstructionRouter) History(limit int) []Result {
	r.mu.Lock()
	defer r.mu.Unlock()

	if limit <= 0 || limit > len(r.history) {
		limit = len(r.history)
	}
	start := len(r.history) - limit
	if start < 0 {
		start = 0
	}
	items := make([]Result, 0, limit)
	for _, result := range r.history[start:] {
		items = append(items, cloneResult(result))
	}
	return items
}

func (r *InstructionRouter) Metrics() map[string]Metrics {
	r.mu.Lock()
	defer r.mu.Unlock()

	snapshot := make(map[string]Metrics, len(r.metrics))
	for instructionType, metrics := range r.metrics {
		snapshot[instructionType] = metrics
	}
	return snapshot
}

func (r *InstructionRouter) Cancel(id string) error {
	r.mu.Lock()
	if item, ok := r.pendingByID[id]; ok {
		heap.Remove(&r.pending, item.index)
		delete(r.pendingByID, id)
		r.recordResultLocked(item.request, item.definition, Result{
			ID:          item.request.ID,
			Type:        item.request.Type,
			Target:      item.definition.Target,
			Status:      StatusCancelled,
			StartedAt:   r.now(),
			CompletedAt: r.now(),
		})
		r.mu.Unlock()
		return nil
	}

	cancel, ok := r.active[id]
	r.mu.Unlock()
	if ok {
		cancel()
		return nil
	}
	return ErrInstructionNotFound
}

func (r *InstructionRouter) executePrepared(ctx context.Context, req Request, definition Definition) (result Result, err error) {
	startedAt := r.now()
	result = Result{
		ID:        req.ID,
		Type:      req.Type,
		Target:    definition.Target,
		Status:    StatusRunning,
		Metadata:  cloneStringMap(req.Metadata),
		StartedAt: startedAt,
	}

	execCtx, cancel := context.WithCancel(ctx)
	if timeout := resolveTimeout(req, definition); timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	r.mu.Lock()
	r.active[req.ID] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.active, req.ID)
		r.mu.Unlock()
	}()

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("instruction handler panic: %v", recovered)
			result.Status = StatusFailed
			result.Error = err.Error()
			result.CompletedAt = r.now()
			result.Duration = result.CompletedAt.Sub(result.StartedAt)
			result = r.recordResult(req, definition, result)
			return
		}

		result.CompletedAt = r.now()
		result.Duration = result.CompletedAt.Sub(result.StartedAt)
		switch {
		case err == nil:
			result.Status = StatusCompleted
		case errors.Is(err, context.Canceled):
			result.Status = StatusCancelled
			result.Error = err.Error()
		default:
			result.Status = StatusFailed
			result.Error = err.Error()
		}
		result = r.recordResult(req, definition, result)
		r.captureShortTermMemory(req, result)
	}()

	output, execErr := definition.Handler.Handle(execCtx, req)
	if execErr != nil {
		err = execErr
		return result, err
	}
	result.Output = cloneMap(output)
	return result, nil
}

func (r *InstructionRouter) prepareRequest(req Request) (Request, Definition, error) {
	definition, err := r.lookupDefinition(req.Type)
	if err != nil {
		return req, Definition{}, err
	}

	prepared := req
	prepared.Type = strings.TrimSpace(prepared.Type)
	if prepared.Type == "" {
		return prepared, definition, fmt.Errorf("instruction type is required")
	}
	if strings.TrimSpace(prepared.ID) == "" {
		prepared.ID = fmt.Sprintf("%s-%d", prepared.Type, atomic.AddUint64(&r.sequence, 1))
	}
	if prepared.Priority == 0 {
		prepared.Priority = definition.DefaultPriority
	}
	if len(prepared.Dependencies) > 0 {
		prepared.Dependencies = append([]string(nil), prepared.Dependencies...)
	}
	if prepared.Payload != nil {
		prepared.Payload = cloneMap(prepared.Payload)
	}
	if prepared.Metadata != nil {
		prepared.Metadata = cloneStringMap(prepared.Metadata)
	}
	if definition.Validator != nil {
		if err := definition.Validator.Validate(prepared); err != nil {
			return prepared, definition, err
		}
	}
	return prepared, definition, nil
}

func (r *InstructionRouter) lookupDefinition(instructionType string) (Definition, error) {
	name := strings.TrimSpace(instructionType)
	if name == "" {
		return Definition{}, fmt.Errorf("instruction type is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	definition, ok := r.definitions[name]
	if !ok {
		return Definition{}, fmt.Errorf("%w: %s", ErrInstructionNotRegistered, name)
	}
	return definition, nil
}

func (r *InstructionRouter) evaluateDependencies(dependencies []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	waiting, err := r.evaluateDependenciesLocked(dependencies)
	if err != nil {
		return err
	}
	if waiting {
		return fmt.Errorf("instruction dependencies are not yet satisfied")
	}
	return err
}

func (r *InstructionRouter) evaluateDependenciesLocked(dependencies []string) (bool, error) {
	for _, dependencyID := range dependencies {
		dependencyID = strings.TrimSpace(dependencyID)
		if dependencyID == "" {
			continue
		}
		if dependency, ok := r.completed[dependencyID]; ok {
			if dependency.Status != StatusCompleted {
				return false, fmt.Errorf("%w: %s", ErrDependencyFailed, dependencyID)
			}
			continue
		}
		if _, ok := r.pendingByID[dependencyID]; ok {
			return true, nil
		}
		if _, ok := r.active[dependencyID]; ok {
			return true, nil
		}
		return true, nil
	}
	return false, nil
}

func (r *InstructionRouter) finishWithoutExecution(req Request, definition Definition, status Status, err error) Result {
	startedAt := r.now()
	result := Result{
		ID:          req.ID,
		Type:        req.Type,
		Status:      status,
		Metadata:    cloneStringMap(req.Metadata),
		StartedAt:   startedAt,
		CompletedAt: startedAt,
	}
	if definition.Target != "" {
		result.Target = definition.Target
	}
	if err != nil {
		result.Error = err.Error()
	}
	return r.recordResult(req, definition, result)
}

func (r *InstructionRouter) recordResult(req Request, definition Definition, result Result) Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recordResultLocked(req, definition, result)
}

func (r *InstructionRouter) recordResultLocked(req Request, definition Definition, result Result) Result {
	recorded := cloneResult(result)
	recorded.Output = cloneMap(result.Output)
	recorded.Metadata = cloneStringMap(result.Metadata)
	r.completed[req.ID] = recorded
	r.history = append(r.history, recorded)
	if len(r.history) > r.historyCap {
		r.history = append([]Result(nil), r.history[len(r.history)-r.historyCap:]...)
	}

	metrics := r.metrics[req.Type]
	metrics.Total++
	metrics.TotalDuration += recorded.Duration
	metrics.LastDuration = recorded.Duration
	metrics.LastError = recorded.Error
	metrics.LastStatus = recorded.Status
	switch recorded.Status {
	case StatusCompleted:
		metrics.Successes++
	case StatusCancelled:
		metrics.Cancelled++
	default:
		metrics.Failures++
	}
	r.metrics[req.Type] = metrics
	return recorded
}

func resolveTimeout(req Request, definition Definition) time.Duration {
	if req.Timeout > 0 {
		return req.Timeout
	}
	return definition.DefaultTimeout
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			cloned[key] = cloneMap(typed)
		case []string:
			cloned[key] = append([]string(nil), typed...)
		case []any:
			items := make([]any, len(typed))
			copy(items, typed)
			cloned[key] = items
		default:
			cloned[key] = value
		}
	}
	return cloned
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func cloneResult(result Result) Result {
	cloned := result
	cloned.Output = cloneMap(result.Output)
	cloned.Metadata = cloneStringMap(result.Metadata)
	return cloned
}

func (r *InstructionRouter) captureShortTermMemory(req Request, result Result) {
	r.mu.Lock()
	store := r.shortTerm
	r.mu.Unlock()
	if store == nil {
		return
	}

	scope := resolveShortTermScope(req.Metadata)
	if scope == "" {
		return
	}

	content := formatShortTermEntry(req, result)
	if content == "" {
		return
	}

	metadata := cloneStringMap(req.Metadata)
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["instruction_id"] = req.ID
	metadata["instruction_type"] = req.Type
	metadata["instruction_status"] = string(result.Status)

	importance := 0.6
	if result.Status == StatusFailed || result.Status == StatusCancelled {
		importance = 0.8
	}

	_, _ = store.Store(memorypkg.StoreInput{
		Scope:      scope,
		ID:         req.ID,
		Kind:       "instruction",
		Content:    content,
		Importance: importance,
		Metadata:   metadata,
	})
}

func resolveShortTermScope(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	for _, key := range []string{"memory_scope", "session_id", "task_id", "project_id"} {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}

func formatShortTermEntry(req Request, result Result) string {
	payload, _ := json.Marshal(req.Payload)
	output, _ := json.Marshal(result.Output)
	builder := strings.Builder{}
	builder.WriteString(req.Type)
	builder.WriteString(" ")
	builder.WriteString(string(result.Status))
	if len(payload) > 0 && string(payload) != "null" && string(payload) != "{}" {
		builder.WriteString(" payload=")
		builder.Write(payload)
	}
	if len(output) > 0 && string(output) != "null" && string(output) != "{}" {
		builder.WriteString(" output=")
		builder.Write(output)
	}
	if result.Error != "" {
		builder.WriteString(" error=")
		builder.WriteString(result.Error)
	}
	return strings.TrimSpace(builder.String())
}
