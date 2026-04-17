package liveartifact

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// stubTaskListReader serves canned List results and records the last
// query it received. Results may be swapped mid-test by reassigning
// tasks/total fields.
type stubTaskListReader struct {
	tasks    []*model.Task
	total    int
	err      error
	lastQ    model.TaskListQuery
	callCnt  int
}

func (s *stubTaskListReader) List(_ context.Context, _ uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error) {
	s.callCnt++
	s.lastQ = q
	if s.err != nil {
		return nil, 0, s.err
	}
	return s.tasks, s.total, nil
}

func taskGroupViewer() model.PrincipalContext {
	return model.PrincipalContext{UserID: uuid.New(), ProjectRole: "viewer"}
}

func inlineTaskRef(t *testing.T, filter map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"kind":   "task_group",
		"filter": filter,
	})
	if err != nil {
		t.Fatalf("marshal inline ref: %v", err)
	}
	return b
}

func savedViewRef(t *testing.T, id string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"kind": "task_group",
		"filter": map[string]any{
			"saved_view_id": id,
		},
	})
	if err != nil {
		t.Fatalf("marshal saved-view ref: %v", err)
	}
	return b
}

func mkTask(title, status string, opts ...func(*model.Task)) *model.Task {
	t := &model.Task{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Title:     title,
		Status:    status,
		Priority:  "medium",
		UpdatedAt: time.Now().Add(-30 * time.Minute),
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

func flattenBlockTexts(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		t.Fatalf("projection not a block array: %v", err)
	}
	var sb strings.Builder
	for _, b := range blocks {
		content, ok := b["content"].([]any)
		if !ok {
			continue
		}
		for _, item := range content {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if s, ok := m["text"].(string); ok {
				sb.WriteString(s)
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func countRowBlocks(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		t.Fatalf("projection not a block array: %v", err)
	}
	n := 0
	for _, b := range blocks {
		if b["type"] != "paragraph" {
			continue
		}
		content, ok := b["content"].([]any)
		if !ok || len(content) == 0 {
			continue
		}
		m, ok := content[0].(map[string]any)
		if !ok {
			continue
		}
		text, _ := m["text"].(string)
		if strings.HasPrefix(text, "• ") {
			n++
		}
	}
	return n
}

func TestTaskGroupProjectorSavedViewUnsupported(t *testing.T) {
	p := NewTaskGroupProjector(&stubTaskListReader{}, nil)
	res, err := p.Project(
		context.Background(),
		taskGroupViewer(),
		uuid.New(),
		savedViewRef(t, uuid.NewString()),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusDegraded {
		t.Fatalf("want StatusDegraded, got %s", res.Status)
	}
	if !strings.Contains(res.Diagnostics, "saved views") {
		t.Fatalf("expected saved-view diagnostic, got %q", res.Diagnostics)
	}
}

func TestTaskGroupProjectorInlineFilterStatus(t *testing.T) {
	tasks := []*model.Task{
		mkTask("One", "in_progress"),
		mkTask("Two", "in_progress"),
		mkTask("Three", "in_progress"),
	}
	stub := &stubTaskListReader{tasks: tasks, total: 3}
	p := NewTaskGroupProjector(stub, nil)

	res, err := p.Project(
		context.Background(),
		taskGroupViewer(),
		uuid.New(),
		inlineTaskRef(t, map[string]any{"status": "in_progress"}),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s (diag=%s)", res.Status, res.Diagnostics)
	}
	if stub.lastQ.Status != "in_progress" {
		t.Fatalf("status filter not forwarded to repo query: %+v", stub.lastQ)
	}
	if got := countRowBlocks(t, res.Projection); got != 3 {
		t.Fatalf("want 3 row blocks, got %d (text=%s)", got, flattenBlockTexts(t, res.Projection))
	}
}

func TestTaskGroupProjectorInlineFilterAssignee(t *testing.T) {
	assignee := uuid.New()
	tasks := []*model.Task{
		mkTask("A", "assigned", func(t *model.Task) { t.AssigneeID = &assignee }),
		mkTask("B", "assigned", func(t *model.Task) { t.AssigneeID = &assignee }),
	}
	stub := &stubTaskListReader{tasks: tasks, total: 2}
	p := NewTaskGroupProjector(stub, nil)

	res, err := p.Project(
		context.Background(),
		taskGroupViewer(),
		uuid.New(),
		inlineTaskRef(t, map[string]any{"assignee": assignee.String()}),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	if stub.lastQ.AssigneeID != assignee.String() {
		t.Fatalf("assignee filter not forwarded: %+v", stub.lastQ)
	}
	if got := countRowBlocks(t, res.Projection); got != 2 {
		t.Fatalf("want 2 rows, got %d", got)
	}
}

func TestTaskGroupProjectorInlineFilterTagPostFilters(t *testing.T) {
	tasks := []*model.Task{
		mkTask("A", "in_progress", func(t *model.Task) { t.Labels = []string{"backend"} }),
		mkTask("B", "in_progress", func(t *model.Task) { t.Labels = []string{"frontend"} }),
		mkTask("C", "in_progress", func(t *model.Task) { t.Labels = []string{"backend", "infra"} }),
		mkTask("D", "in_progress", func(t *model.Task) { t.Labels = []string{"docs"} }),
	}
	stub := &stubTaskListReader{tasks: tasks, total: 4}
	p := NewTaskGroupProjector(stub, nil)

	res, err := p.Project(
		context.Background(),
		taskGroupViewer(),
		uuid.New(),
		inlineTaskRef(t, map[string]any{"tag": "backend"}),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	if got := countRowBlocks(t, res.Projection); got != 2 {
		t.Fatalf("want 2 rows after tag post-filter, got %d (text=%s)", got, flattenBlockTexts(t, res.Projection))
	}
}

func TestTaskGroupProjectorTruncationFooter(t *testing.T) {
	tasks := make([]*model.Task, 0, 60)
	for i := 0; i < 60; i++ {
		tasks = append(tasks, mkTask("T", "in_progress"))
	}
	stub := &stubTaskListReader{tasks: tasks, total: 75}
	p := NewTaskGroupProjector(stub, nil)

	res, err := p.Project(
		context.Background(),
		taskGroupViewer(),
		uuid.New(),
		inlineTaskRef(t, map[string]any{"status": "in_progress"}),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	if got := countRowBlocks(t, res.Projection); got != 50 {
		t.Fatalf("want 50 rows after cap, got %d", got)
	}
	text := flattenBlockTexts(t, res.Projection)
	if !strings.Contains(text, "25 more matching tasks") {
		t.Fatalf("expected truncation footer, got: %s", text)
	}
}

func TestTaskGroupProjectorEmpty(t *testing.T) {
	stub := &stubTaskListReader{tasks: nil, total: 0}
	p := NewTaskGroupProjector(stub, nil)

	res, err := p.Project(
		context.Background(),
		taskGroupViewer(),
		uuid.New(),
		inlineTaskRef(t, map[string]any{"status": "done"}),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	text := flattenBlockTexts(t, res.Projection)
	if !strings.Contains(text, "No tasks match this filter.") {
		t.Fatalf("expected empty-state paragraph, got: %s", text)
	}
}

func TestTaskGroupProjectorInvalidTargetRef(t *testing.T) {
	p := NewTaskGroupProjector(&stubTaskListReader{}, nil)

	// Neither saved_view_id nor inline fields set.
	res, err := p.Project(
		context.Background(),
		taskGroupViewer(),
		uuid.New(),
		inlineTaskRef(t, map[string]any{}),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusDegraded {
		t.Fatalf("want StatusDegraded, got %s", res.Status)
	}
	if res.Diagnostics == "" {
		t.Fatalf("expected diagnostic for invalid target_ref")
	}
}

func TestTaskGroupProjectorForbidden(t *testing.T) {
	p := NewTaskGroupProjector(&stubTaskListReader{}, nil)
	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: ""}
	res, err := p.Project(
		context.Background(),
		pc,
		uuid.New(),
		inlineTaskRef(t, map[string]any{"status": "in_progress"}),
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusForbidden {
		t.Fatalf("want StatusForbidden, got %s", res.Status)
	}
	if len(res.Projection) != 0 {
		t.Fatalf("forbidden result must not leak projection")
	}
}

func TestTaskGroupProjectorSubscribeUnfiltered(t *testing.T) {
	p := NewTaskGroupProjector(&stubTaskListReader{}, nil)
	topics := p.Subscribe(inlineTaskRef(t, map[string]any{"status": "in_progress"}))
	if len(topics) != 5 {
		t.Fatalf("want 5 topics, got %d", len(topics))
	}
	want := map[string]struct{}{
		"task.created":      {},
		"task.updated":      {},
		"task.deleted":      {},
		"task.transitioned": {},
		"task.assigned":     {},
	}
	for _, topic := range topics {
		if _, ok := want[topic.Event]; !ok {
			t.Errorf("unexpected event %q", topic.Event)
		}
		if topic.Scope["project_id"] != scopeAssetProject {
			t.Errorf("topic %q missing project_id scope: %+v", topic.Event, topic.Scope)
		}
		if _, ok := topic.Scope["sprint_id"]; ok {
			t.Errorf("unexpected sprint_id scope for unfiltered topic %q", topic.Event)
		}
	}
}

func TestTaskGroupProjectorSubscribeWithSprintFilter(t *testing.T) {
	sprintID := uuid.New().String()
	p := NewTaskGroupProjector(&stubTaskListReader{}, nil)
	topics := p.Subscribe(inlineTaskRef(t, map[string]any{"sprint_id": sprintID}))
	if len(topics) != 5 {
		t.Fatalf("want 5 topics, got %d", len(topics))
	}
	for _, topic := range topics {
		if topic.Scope["project_id"] != scopeAssetProject {
			t.Errorf("topic %q missing project_id scope", topic.Event)
		}
		if topic.Scope["sprint_id"] != sprintID {
			t.Errorf("topic %q missing sprint_id scope: %+v", topic.Event, topic.Scope)
		}
	}
}

func TestTaskGroupProjectorReflectsStatusChangeAcrossCalls(t *testing.T) {
	task := mkTask("X", "in_progress")
	stub := &stubTaskListReader{tasks: []*model.Task{task}, total: 1}
	p := NewTaskGroupProjector(stub, nil)

	ref := inlineTaskRef(t, map[string]any{"status": "in_progress"})

	res1, err := p.Project(context.Background(), taskGroupViewer(), uuid.New(), ref, nil)
	if err != nil || res1.Status != StatusOK {
		t.Fatalf("first projection: status=%s err=%v", res1.Status, err)
	}
	if !strings.Contains(flattenBlockTexts(t, res1.Projection), "in_progress") {
		t.Fatalf("first projection missing initial status")
	}

	// Mutate the stub to simulate a status change arriving through the WS
	// re-project path.
	updated := mkTask("X", "in_review")
	updated.ID = task.ID
	stub.tasks = []*model.Task{updated}

	res2, err := p.Project(context.Background(), taskGroupViewer(), uuid.New(), ref, nil)
	if err != nil || res2.Status != StatusOK {
		t.Fatalf("second projection: status=%s err=%v", res2.Status, err)
	}
	text := flattenBlockTexts(t, res2.Projection)
	// Row-level check: the task row ("• X — <status> · ...") must carry
	// the new status, not the old one. The heading may still echo the
	// filter value "status=in_progress" — that's a filter label, not a
	// cached task state.
	if !strings.Contains(text, "• X — in_review") {
		t.Fatalf("second projection row missing new status: %s", text)
	}
	if strings.Contains(text, "• X — in_progress") {
		t.Fatalf("second projection row leaked previous status: %s", text)
	}
	if stub.callCnt != 2 {
		t.Fatalf("expected 2 repo calls, got %d", stub.callCnt)
	}
}
