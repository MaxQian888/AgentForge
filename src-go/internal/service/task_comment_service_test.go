package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type stubTaskCommentRepo struct {
	comments map[uuid.UUID]*model.TaskComment
}

func (r *stubTaskCommentRepo) Create(_ context.Context, comment *model.TaskComment) error {
	cloned := *comment
	if r.comments == nil {
		r.comments = make(map[uuid.UUID]*model.TaskComment)
	}
	r.comments[comment.ID] = &cloned
	return nil
}

func (r *stubTaskCommentRepo) GetByID(_ context.Context, id uuid.UUID) (*model.TaskComment, error) {
	comment, ok := r.comments[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	cloned := *comment
	return &cloned, nil
}

func (r *stubTaskCommentRepo) Update(_ context.Context, comment *model.TaskComment) error {
	cloned := *comment
	r.comments[comment.ID] = &cloned
	return nil
}

func (r *stubTaskCommentRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	comment := r.comments[id]
	now := time.Now().UTC()
	comment.DeletedAt = &now
	return nil
}

func (r *stubTaskCommentRepo) ListByTaskID(_ context.Context, taskID uuid.UUID) ([]*model.TaskComment, error) {
	result := make([]*model.TaskComment, 0)
	for _, comment := range r.comments {
		if comment.TaskID == taskID && comment.DeletedAt == nil {
			cloned := *comment
			result = append(result, &cloned)
		}
	}
	return result, nil
}

type stubTaskCommentMemberRepo struct {
	members []*model.Member
}

func (r *stubTaskCommentMemberRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.Member, error) {
	result := make([]*model.Member, 0, len(r.members))
	for _, member := range r.members {
		if member.ProjectID == projectID {
			cloned := *member
			result = append(result, &cloned)
		}
	}
	return result, nil
}

type stubTaskCommentNotifier struct {
	targets []uuid.UUID
	types   []string
}

func (n *stubTaskCommentNotifier) Create(_ context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error) {
	_ = title
	_ = body
	_ = data
	n.targets = append(n.targets, targetID)
	n.types = append(n.types, ntype)
	return &model.Notification{ID: uuid.New(), TargetID: targetID, Type: ntype}, nil
}

func TestTaskCommentServiceCreateReplyResolveReopenDelete(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	repo := &stubTaskCommentRepo{}
	memberRepo := &stubTaskCommentMemberRepo{
		members: []*model.Member{
			{ID: uuid.New(), ProjectID: projectID, Name: "alice"},
		},
	}
	notifier := &stubTaskCommentNotifier{}
	svc := NewTaskCommentService(repo, memberRepo, notifier)

	comment, err := svc.CreateComment(context.Background(), &CreateTaskCommentInput{
		ProjectID: projectID,
		TaskID:    taskID,
		Body:      "Need @alice on this task",
		CreatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}
	if len(comment.Mentions) != 1 || comment.Mentions[0] != "alice" {
		t.Fatalf("Mentions = %v, want [alice]", comment.Mentions)
	}
	if len(notifier.targets) != 1 {
		t.Fatalf("len(notifier.targets) = %d, want 1", len(notifier.targets))
	}

	reply, err := svc.ReplyToComment(context.Background(), comment.ID, &CreateTaskCommentInput{
		ProjectID: projectID,
		TaskID:    taskID,
		Body:      "reply",
		CreatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("ReplyToComment() error = %v", err)
	}
	if reply.ParentCommentID == nil || *reply.ParentCommentID != comment.ID {
		t.Fatalf("ParentCommentID = %v, want %s", reply.ParentCommentID, comment.ID)
	}

	resolved, err := svc.ResolveComment(context.Background(), comment.ID)
	if err != nil {
		t.Fatalf("ResolveComment() error = %v", err)
	}
	if resolved.ResolvedAt == nil {
		t.Fatal("expected ResolvedAt to be populated")
	}

	reopened, err := svc.ReopenComment(context.Background(), comment.ID)
	if err != nil {
		t.Fatalf("ReopenComment() error = %v", err)
	}
	if reopened.ResolvedAt != nil {
		t.Fatal("expected ResolvedAt to be cleared")
	}

	list, err := svc.ListComments(context.Background(), taskID)
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(ListComments()) = %d, want 2", len(list))
	}

	if err := svc.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatalf("DeleteComment() error = %v", err)
	}
	list, err = svc.ListComments(context.Background(), taskID)
	if err != nil {
		t.Fatalf("ListComments() after delete error = %v", err)
	}
	if len(list) != 1 || list[0].ID != reply.ID {
		t.Fatalf("ListComments() after delete = %+v, want only reply %s", list, reply.ID)
	}
}

func TestExtractTaskCommentMentions(t *testing.T) {
	mentions := ExtractTaskCommentMentions("Ping @alice and @bob-builder, skip email@example.com")
	if len(mentions) != 2 || mentions[0] != "alice" || mentions[1] != "bob-builder" {
		t.Fatalf("ExtractTaskCommentMentions() = %v, want [alice bob-builder]", mentions)
	}
}

func TestTaskCommentServiceBroadcastsLifecycleEvents(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	repo := &stubTaskCommentRepo{}
	memberRepo := &stubTaskCommentMemberRepo{}
	hub := ws.NewHub()
	eventCh := attachTaskCommentProjectListener(t, hub, projectID.String())
	taskRepo := &stubEntityLinkTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			taskID: {ID: taskID, ProjectID: projectID, Title: "Task"},
		},
	}
	svc := NewTaskCommentService(repo, memberRepo, nil, taskRepo).WithHub(hub)

	comment, err := svc.CreateComment(context.Background(), &CreateTaskCommentInput{
		ProjectID: projectID,
		TaskID:    taskID,
		Body:      "hello",
		CreatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}
	if _, err := svc.ResolveComment(context.Background(), comment.ID); err != nil {
		t.Fatalf("ResolveComment() error = %v", err)
	}

	created := decodeTaskCommentEvent(t, <-eventCh)
	resolved := decodeTaskCommentEvent(t, <-eventCh)
	if created.Type != ws.EventTaskCommentCreated {
		t.Fatalf("created event type = %q, want %q", created.Type, ws.EventTaskCommentCreated)
	}
	if resolved.Type != ws.EventTaskCommentResolved {
		t.Fatalf("resolved event type = %q, want %q", resolved.Type, ws.EventTaskCommentResolved)
	}
}

func attachTaskCommentProjectListener(t *testing.T, hub *ws.Hub, projectID string) chan []byte {
	t.Helper()

	client := &ws.Client{}
	send := make(chan []byte, 8)

	clientValue := reflect.ValueOf(client).Elem()
	projectField := clientValue.FieldByName("projectID")
	reflect.NewAt(projectField.Type(), unsafe.Pointer(projectField.UnsafeAddr())).Elem().SetString(projectID)
	sendField := clientValue.FieldByName("send")
	reflect.NewAt(sendField.Type(), unsafe.Pointer(sendField.UnsafeAddr())).Elem().Set(reflect.ValueOf(send))

	hubValue := reflect.ValueOf(hub).Elem()
	clientsField := hubValue.FieldByName("clients")
	clientsMap := reflect.NewAt(clientsField.Type(), unsafe.Pointer(clientsField.UnsafeAddr())).Elem()
	clientsMap.SetMapIndex(reflect.ValueOf(client), reflect.ValueOf(struct{}{}))

	return send
}

func decodeTaskCommentEvent(t *testing.T, raw []byte) ws.Event {
	t.Helper()

	var event ws.Event
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("unmarshal ws event: %v", err)
	}
	return event
}
