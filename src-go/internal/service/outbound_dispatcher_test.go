package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type fakeExecRepo struct {
	exec *model.WorkflowExecution
}

func (f *fakeExecRepo) GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error) {
	return f.exec, nil
}

type captureBus struct {
	onPublish func(*eb.Event)
}

func (b *captureBus) Publish(ctx context.Context, e *eb.Event) error {
	if b.onPublish != nil {
		b.onPublish(e)
	}
	return nil
}

func mkExec(t *testing.T, status string, sysMeta map[string]any) *model.WorkflowExecution {
	t.Helper()
	raw, _ := json.Marshal(sysMeta)
	return &model.WorkflowExecution{
		ID:             uuid.New(),
		WorkflowID:     uuid.New(),
		ProjectID:      uuid.New(),
		Status:         status,
		SystemMetadata: raw,
	}
}

func TestDispatcher_Matrix(t *testing.T) {
	cases := []struct {
		name         string
		replyTarget  map[string]any
		imDispatched bool
		wantPosts    int32
	}{
		{"no reply target", nil, false, 0},
		{"reply target + im_dispatched", map[string]any{"platform": "feishu", "chat_id": "c"}, true, 0},
		{"reply target + not dispatched (success)", map[string]any{"platform": "feishu", "chat_id": "c"}, false, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var posts int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&posts, 1)
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"status":"sent"}`))
			}))
			defer srv.Close()
			meta := map[string]any{}
			if c.replyTarget != nil {
				meta["reply_target"] = c.replyTarget
			}
			if c.imDispatched {
				meta["im_dispatched"] = true
			}
			exec := mkExec(t, model.WorkflowExecStatusCompleted, meta)
			d := service.NewOutboundDispatcher(&fakeExecRepo{exec: exec}, srv.URL, "https://fe.example", nil)
			d.SetRetryDelays(0, 0, 0)
			payload, _ := json.Marshal(map[string]any{
				"executionId": exec.ID.String(),
				"workflowId":  exec.WorkflowID.String(),
				"status":      model.WorkflowExecStatusCompleted,
			})
			ev := eb.NewEvent(ws.EventWorkflowExecutionCompleted, "core", "project:"+exec.ProjectID.String())
			ev.Payload = payload
			d.Observe(context.Background(), ev, &eb.PipelineCtx{})
			// Allow goroutine dispatch + http roundtrip to complete.
			deadline := time.Now().Add(1 * time.Second)
			for time.Now().Before(deadline) && atomic.LoadInt32(&posts) < c.wantPosts {
				time.Sleep(10 * time.Millisecond)
			}
			// And one extra delay so a falsely-firing dispatch has a chance to be observed.
			time.Sleep(50 * time.Millisecond)
			if got := atomic.LoadInt32(&posts); got != c.wantPosts {
				t.Fatalf("posts: want %d got %d", c.wantPosts, got)
			}
		})
	}
}

func TestDispatcher_RetriesThenEmitsFailureEvent(t *testing.T) {
	var posts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&posts, 1)
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()

	exec := mkExec(t, model.WorkflowExecStatusFailed, map[string]any{
		"reply_target": map[string]any{"platform": "feishu", "chat_id": "c"},
	})
	var emitted []eb.Event
	bus := &captureBus{onPublish: func(e *eb.Event) { emitted = append(emitted, *e) }}
	d := service.NewOutboundDispatcher(&fakeExecRepo{exec: exec}, srv.URL, "https://fe.example", bus)
	d.SetRetryDelays(0, 0, 0)

	payload, _ := json.Marshal(map[string]any{
		"executionId": exec.ID.String(),
		"status":      model.WorkflowExecStatusFailed,
	})
	ev := eb.NewEvent(eb.EventWorkflowExecutionCompleted, "core", "project:"+exec.ProjectID.String())
	ev.Payload = payload
	d.Observe(context.Background(), ev, &eb.PipelineCtx{})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && (atomic.LoadInt32(&posts) < 3 || len(emitted) == 0) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&posts); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
	if len(emitted) != 1 || emitted[0].Type != eb.EventOutboundDeliveryFailed {
		t.Fatalf("expected one EventOutboundDeliveryFailed, got %+v", emitted)
	}
}
