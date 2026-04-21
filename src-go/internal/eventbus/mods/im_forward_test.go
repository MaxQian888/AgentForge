package mods

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeAncestorResolver struct {
	// root maps taskID → root task; if absent, the task is its own root.
	root map[uuid.UUID]*model.Task
	err  error
}

func (f *fakeAncestorResolver) GetAncestorRoot(_ context.Context, taskID uuid.UUID) (*model.Task, error) {
	if f.err != nil {
		return nil, f.err
	}
	if r, ok := f.root[taskID]; ok {
		return r, nil
	}
	// Default: task is its own root.
	return &model.Task{ID: taskID}, nil
}

type fakeIMCP struct {
	// platform keyed by taskID string; "" means no binding.
	platformByTask map[string]string
	dispatched     []fakeDispatch
	queueErr       error
}

type fakeDispatch struct {
	taskID  string
	content string
}

func (f *fakeIMCP) BoundPlatformForTask(taskID string) string {
	if f.platformByTask == nil {
		return ""
	}
	return f.platformByTask[taskID]
}

func (f *fakeIMCP) QueueBoundProgressRaw(_ context.Context, taskID, content string, _ bool, _ map[string]string) (bool, error) {
	if f.queueErr != nil {
		return false, f.queueErr
	}
	f.dispatched = append(f.dispatched, fakeDispatch{taskID: taskID, content: content})
	return true, nil
}

// --- helpers ---

func makeTaskEvent(typ, taskIDStr string, projectID string) *eb.Event {
	e := eb.NewEvent(typ, "core", "task:"+taskIDStr)
	if projectID != "" {
		eb.SetString(e, eb.MetaProjectID, projectID)
	}
	return e
}

// --- tests ---

func TestIMForward_NilControlPlaneIsNoop(t *testing.T) {
	m := NewIMForward(nil, nil)
	// Must not panic.
	m.Observe(context.Background(), makeTaskEvent("task.created", uuid.New().String(), ""), &eb.PipelineCtx{})
}

func TestIMForward_NilEventIsNoop(t *testing.T) {
	cp := &fakeIMCP{}
	m := NewIMForward(nil, cp)
	m.Observe(context.Background(), nil, &eb.PipelineCtx{})
	assert.Empty(t, cp.dispatched)
}

func TestIMForward_NonTaskTargetIsIgnored(t *testing.T) {
	cp := &fakeIMCP{}
	m := NewIMForward(nil, cp)
	e := eb.NewEvent("task.created", "core", "project:p1")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Empty(t, cp.dispatched)
}

func TestIMForward_RootTaskWithBinding_PostsToIM(t *testing.T) {
	rootID := uuid.New()
	cp := &fakeIMCP{
		platformByTask: map[string]string{rootID.String(): "slack"},
	}
	m := NewIMForward(nil, cp) // no resolver: task is its own root

	e := makeTaskEvent("task.completed", rootID.String(), "proj-1")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})

	require.Len(t, cp.dispatched, 1)
	assert.Equal(t, rootID.String(), cp.dispatched[0].taskID)
}

func TestIMForward_RootTaskNoBinding_SkipsIM(t *testing.T) {
	rootID := uuid.New()
	cp := &fakeIMCP{platformByTask: map[string]string{}} // no binding
	m := NewIMForward(nil, cp)

	e := makeTaskEvent("task.created", rootID.String(), "")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Empty(t, cp.dispatched)
}

func TestIMForward_ChildTask_NestedMode_PostsToRootBinding(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()

	resolver := &fakeAncestorResolver{
		root: map[uuid.UUID]*model.Task{
			childID: {ID: rootID},
		},
	}
	cp := &fakeIMCP{
		platformByTask: map[string]string{rootID.String(): "feishu"},
	}
	m := NewIMForward(resolver, cp)

	e := makeTaskEvent("agent.started", childID.String(), "proj-1")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})

	require.Len(t, cp.dispatched, 1)
	assert.Equal(t, rootID.String(), cp.dispatched[0].taskID, "should use root task's binding")
	assert.Contains(t, cp.dispatched[0].content, "[sub-agent ", "should include sub-agent prefix")
}

func TestIMForward_ChildTask_FrontendOnlyMode_SkipsIM(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()

	resolver := &fakeAncestorResolver{
		root: map[uuid.UUID]*model.Task{
			childID: {ID: rootID},
		},
	}
	cp := &fakeIMCP{
		platformByTask: map[string]string{rootID.String(): "qq"},
	}
	m := NewIMForward(resolver, cp)

	e := makeTaskEvent("agent.started", childID.String(), "")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Empty(t, cp.dispatched, "frontend_only: child events should not post to IM")
}

func TestIMForward_RootTask_FrontendOnlyMode_PostsNormally(t *testing.T) {
	rootID := uuid.New()
	cp := &fakeIMCP{
		platformByTask: map[string]string{rootID.String(): "qqbot"},
	}
	m := NewIMForward(nil, cp)

	e := makeTaskEvent("task.completed", rootID.String(), "")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	require.Len(t, cp.dispatched, 1)
	assert.Equal(t, rootID.String(), cp.dispatched[0].taskID)
}

func TestIMForward_ChildTask_FlatMode_UsesChildBindingIfPresent(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()

	resolver := &fakeAncestorResolver{
		root: map[uuid.UUID]*model.Task{
			childID: {ID: rootID},
		},
	}
	// Both root and child have bindings; platform is "dingtalk" → flat mode.
	cp := &fakeIMCP{
		platformByTask: map[string]string{
			rootID.String():  "dingtalk",
			childID.String(): "dingtalk",
		},
	}
	m := NewIMForward(resolver, cp)

	e := makeTaskEvent("agent.started", childID.String(), "")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})

	require.Len(t, cp.dispatched, 1)
	assert.Equal(t, childID.String(), cp.dispatched[0].taskID, "flat: should use child's own binding")
}

func TestIMForward_ChildTask_FlatMode_FallsBackToRootIfNoChildBinding(t *testing.T) {
	rootID := uuid.New()
	childID := uuid.New()

	resolver := &fakeAncestorResolver{
		root: map[uuid.UUID]*model.Task{
			childID: {ID: rootID},
		},
	}
	// Only the root has a binding; platform is "email" (unknown → flat).
	cp := &fakeIMCP{
		platformByTask: map[string]string{rootID.String(): "email"},
	}
	m := NewIMForward(resolver, cp)

	e := makeTaskEvent("task.completed", childID.String(), "")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})

	require.Len(t, cp.dispatched, 1)
	assert.Equal(t, rootID.String(), cp.dispatched[0].taskID, "flat: should fall back to root binding")
}

func TestIMForward_AncestorLookupError_SkipsIM(t *testing.T) {
	resolver := &fakeAncestorResolver{err: errors.New("db unavailable")}
	cp := &fakeIMCP{platformByTask: map[string]string{"any": "slack"}}
	m := NewIMForward(resolver, cp)

	e := makeTaskEvent("task.created", uuid.New().String(), "")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Empty(t, cp.dispatched)
}

func TestIMForward_MetadataIdentity(t *testing.T) {
	m := NewIMForward(nil, nil)
	assert.Equal(t, "im.forward", m.Name())
	assert.Equal(t, 81, m.Priority())
	assert.Equal(t, eb.ModeObserve, m.Mode())
	assert.Contains(t, m.Intercepts(), "task.*")
}

func TestIMForward_NestedMode_RootEventNoPrefix(t *testing.T) {
	rootID := uuid.New()
	cp := &fakeIMCP{platformByTask: map[string]string{rootID.String(): "slack"}}
	m := NewIMForward(nil, cp)

	e := makeTaskEvent("task.completed", rootID.String(), "proj-1")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})

	require.Len(t, cp.dispatched, 1)
	assert.NotContains(t, cp.dispatched[0].content, "[sub-agent", "root events should not have sub-agent prefix")
}
