package instruction_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/agentforge/server/internal/instruction"
	"github.com/agentforge/server/internal/memory"
)

func TestInstructionRouter_ExecuteRoutesByTarget(t *testing.T) {
	t.Parallel()

	router := instruction.NewRouter()

	var mu sync.Mutex
	calls := make([]instruction.Target, 0, 3)
	register := func(name string, target instruction.Target) {
		t.Helper()
		if err := router.Register(name, instruction.Definition{
			Target: target,
			Handler: instruction.HandlerFunc(func(ctx context.Context, req instruction.Request) (map[string]any, error) {
				mu.Lock()
				calls = append(calls, target)
				mu.Unlock()
				return map[string]any{"instruction": req.Type, "target": string(target)}, nil
			}),
		}); err != nil {
			t.Fatalf("Register(%s) error = %v", name, err)
		}
	}

	register("read", instruction.TargetLocal)
	register("think", instruction.TargetBridge)
	register("plugin.search", instruction.TargetPlugin)

	tests := []struct {
		name   string
		req    instruction.Request
		target instruction.Target
	}{
		{
			name:   "local",
			req:    instruction.Request{ID: "local-1", Type: "read"},
			target: instruction.TargetLocal,
		},
		{
			name:   "bridge",
			req:    instruction.Request{ID: "bridge-1", Type: "think"},
			target: instruction.TargetBridge,
		},
		{
			name:   "plugin",
			req:    instruction.Request{ID: "plugin-1", Type: "plugin.search"},
			target: instruction.TargetPlugin,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := router.Execute(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("Execute(%s) error = %v", tc.req.Type, err)
			}
			if result.Target != tc.target {
				t.Fatalf("result.Target = %s, want %s", result.Target, tc.target)
			}
			if result.Status != instruction.StatusCompleted {
				t.Fatalf("result.Status = %s, want %s", result.Status, instruction.StatusCompleted)
			}
			if got := result.Output["instruction"]; got != tc.req.Type {
				t.Fatalf("result.Output[instruction] = %v, want %s", got, tc.req.Type)
			}
		})
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 3 {
		t.Fatalf("handler calls = %d, want 3", len(calls))
	}
	if calls[0] != instruction.TargetLocal || calls[1] != instruction.TargetBridge || calls[2] != instruction.TargetPlugin {
		t.Fatalf("handler calls = %v, want [local bridge plugin]", calls)
	}
}

func TestInstructionRouter_ValidationAndTimeout(t *testing.T) {
	t.Parallel()

	router := instruction.NewRouter()
	handlerCalls := 0

	if err := router.Register("read", instruction.Definition{
		Target:         instruction.TargetLocal,
		DefaultTimeout: 10 * time.Millisecond,
		Validator: instruction.ValidatorFunc(func(req instruction.Request) error {
			path, _ := req.Payload["path"].(string)
			if path == "" {
				return errors.New("path is required")
			}
			return nil
		}),
		Handler: instruction.HandlerFunc(func(ctx context.Context, req instruction.Request) (map[string]any, error) {
			handlerCalls++
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return map[string]any{"ok": true}, nil
			}
		}),
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if _, err := router.Execute(context.Background(), instruction.Request{
		ID:      "invalid",
		Type:    "read",
		Payload: map[string]any{},
	}); err == nil {
		t.Fatal("Execute() error = nil, want validation error")
	}
	if handlerCalls != 0 {
		t.Fatalf("handlerCalls = %d, want 0", handlerCalls)
	}

	result, err := router.Execute(context.Background(), instruction.Request{
		ID:      "timeout",
		Type:    "read",
		Payload: map[string]any{"path": "README.md"},
	})
	if err == nil {
		t.Fatal("Execute(timeout) error = nil, want timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Execute(timeout) error = %v, want context deadline exceeded", err)
	}
	if result.Status != instruction.StatusFailed {
		t.Fatalf("timeout result.Status = %s, want %s", result.Status, instruction.StatusFailed)
	}
	if handlerCalls != 1 {
		t.Fatalf("handlerCalls = %d, want 1", handlerCalls)
	}
}

func TestInstructionRouter_ExecuteRequiresSatisfiedDependencies(t *testing.T) {
	t.Parallel()

	router := instruction.NewRouter()
	if err := router.Register("read", instruction.Definition{
		Target: instruction.TargetLocal,
		Handler: instruction.HandlerFunc(func(ctx context.Context, req instruction.Request) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		}),
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	result, err := router.Execute(context.Background(), instruction.Request{
		ID:           "needs-dependency",
		Type:         "read",
		Dependencies: []string{"missing"},
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want unsatisfied dependency error")
	}
	if result.Status != instruction.StatusFailed {
		t.Fatalf("result.Status = %s, want %s", result.Status, instruction.StatusFailed)
	}
}

func TestInstructionRouter_QueuePriorityDependenciesAndMetrics(t *testing.T) {
	t.Parallel()

	router := instruction.NewRouter()
	executed := make([]string, 0, 3)

	if err := router.Register("ok", instruction.Definition{
		Target:          instruction.TargetLocal,
		DefaultPriority: 100,
		Handler: instruction.HandlerFunc(func(ctx context.Context, req instruction.Request) (map[string]any, error) {
			executed = append(executed, req.ID)
			return map[string]any{"id": req.ID}, nil
		}),
	}); err != nil {
		t.Fatalf("Register(ok) error = %v", err)
	}
	if err := router.Register("fail", instruction.Definition{
		Target: instruction.TargetBridge,
		Handler: instruction.HandlerFunc(func(ctx context.Context, req instruction.Request) (map[string]any, error) {
			executed = append(executed, req.ID)
			return nil, errors.New("boom")
		}),
	}); err != nil {
		t.Fatalf("Register(fail) error = %v", err)
	}

	for _, req := range []instruction.Request{
		{ID: "low", Type: "ok", Priority: 10},
		{ID: "blocked", Type: "ok", Priority: 1000, Dependencies: []string{"missing"}},
		{ID: "high", Type: "ok", Priority: 100},
		{ID: "base-fail", Type: "fail", Priority: 50},
		{ID: "dependent-fail", Type: "ok", Priority: 40, Dependencies: []string{"base-fail"}},
	} {
		if err := router.Enqueue(req); err != nil {
			t.Fatalf("Enqueue(%s) error = %v", req.ID, err)
		}
	}

	result, err := router.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext(high) error = %v", err)
	}
	if result.ID != "high" {
		t.Fatalf("first result.ID = %s, want high", result.ID)
	}

	result, err = router.ProcessNext(context.Background())
	if err == nil {
		t.Fatal("ProcessNext(base-fail) error = nil, want handler error")
	}
	if result.ID != "base-fail" || result.Status != instruction.StatusFailed {
		t.Fatalf("base-fail result = %#v, want failed base-fail", result)
	}

	result, err = router.ProcessNext(context.Background())
	if err == nil {
		t.Fatal("ProcessNext(dependent-fail) error = nil, want dependency error")
	}
	if result.ID != "dependent-fail" || result.Status != instruction.StatusFailed {
		t.Fatalf("dependent-fail result = %#v, want failed dependency result", result)
	}

	result, err = router.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext(low) error = %v", err)
	}
	if result.ID != "low" {
		t.Fatalf("fourth result.ID = %s, want low", result.ID)
	}

	pending := router.Pending()
	if len(pending) != 1 || pending[0].ID != "blocked" {
		t.Fatalf("Pending() = %#v, want only blocked instruction", pending)
	}

	metrics := router.Metrics()
	if metrics["ok"].Successes != 2 {
		t.Fatalf("metrics[ok].Successes = %d, want 2", metrics["ok"].Successes)
	}
	if metrics["fail"].Failures != 1 {
		t.Fatalf("metrics[fail].Failures = %d, want 1", metrics["fail"].Failures)
	}

	if len(executed) != 3 || executed[0] != "high" || executed[1] != "base-fail" || executed[2] != "low" {
		t.Fatalf("executed = %v, want [high base-fail low]", executed)
	}
}

func TestInstructionRouter_CancelQueuedAndRunning(t *testing.T) {
	t.Parallel()

	router := instruction.NewRouter()

	started := make(chan struct{}, 1)
	if err := router.Register("slow", instruction.Definition{
		Target: instruction.TargetPlugin,
		Handler: instruction.HandlerFunc(func(ctx context.Context, req instruction.Request) (map[string]any, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		}),
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if err := router.Enqueue(instruction.Request{ID: "queued", Type: "slow"}); err != nil {
		t.Fatalf("Enqueue(queued) error = %v", err)
	}
	if err := router.Cancel("queued"); err != nil {
		t.Fatalf("Cancel(queued) error = %v", err)
	}
	if pending := router.Pending(); len(pending) != 0 {
		t.Fatalf("Pending() after cancel = %#v, want empty", pending)
	}

	resultCh := make(chan instruction.Result, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := router.Execute(context.Background(), instruction.Request{ID: "running", Type: "slow"})
		resultCh <- result
		errCh <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	if err := router.Cancel("running"); err != nil {
		t.Fatalf("Cancel(running) error = %v", err)
	}

	result := <-resultCh
	err := <-errCh
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("running error = %v, want context canceled", err)
	}
	if result.Status != instruction.StatusCancelled {
		t.Fatalf("running result.Status = %s, want %s", result.Status, instruction.StatusCancelled)
	}
}

func TestInstructionRouter_StoresCompletedExecutionInShortTermMemory(t *testing.T) {
	t.Parallel()

	store := memory.NewShortTermMemory(memory.Config{
		MaxTokens:            32,
		DefaultContextTokens: 32,
		TokenEstimator: func(text string) int {
			if text == "" {
				return 0
			}
			count := 1
			for _, ch := range text {
				if ch == ' ' {
					count++
				}
			}
			return count
		},
	})
	router := instruction.NewRouter().WithShortTermMemory(store)

	if err := router.Register("read", instruction.Definition{
		Target: instruction.TargetLocal,
		Handler: instruction.HandlerFunc(func(ctx context.Context, req instruction.Request) (map[string]any, error) {
			return map[string]any{"path": "README.md"}, nil
		}),
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if _, err := router.Execute(context.Background(), instruction.Request{
		ID:       "memory-capture",
		Type:     "read",
		Metadata: map[string]string{"session_id": "session-1"},
		Payload:  map[string]any{"path": "README.md"},
	}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	contextEntries, err := store.Context("session-1", 32)
	if err != nil {
		t.Fatalf("Context() error = %v", err)
	}
	if len(contextEntries) != 1 {
		t.Fatalf("len(contextEntries) = %d, want 1", len(contextEntries))
	}
	if contextEntries[0].Metadata["instruction_type"] != "read" {
		t.Fatalf("entry metadata = %#v, want instruction_type=read", contextEntries[0].Metadata)
	}
	if contextEntries[0].Content == "" {
		t.Fatalf("entry content = empty, want summarized execution content")
	}
}
