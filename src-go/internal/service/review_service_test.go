package service_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type mockReviewRepo struct {
	created []*model.Review
	updated []*model.Review
	byID    map[uuid.UUID]*model.Review
}

func newMockReviewRepo() *mockReviewRepo {
	return &mockReviewRepo{byID: make(map[uuid.UUID]*model.Review)}
}

func (m *mockReviewRepo) Create(_ context.Context, review *model.Review) error {
	cloned := *review
	m.created = append(m.created, &cloned)
	m.byID[review.ID] = &cloned
	return nil
}

func (m *mockReviewRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Review, error) {
	review, ok := m.byID[id]
	if !ok {
		return nil, service.ErrReviewNotFound
	}
	cloned := *review
	return &cloned, nil
}

func (m *mockReviewRepo) GetByTask(_ context.Context, taskID uuid.UUID) ([]*model.Review, error) {
	var reviews []*model.Review
	for _, review := range m.byID {
		if review.TaskID == taskID {
			cloned := *review
			reviews = append(reviews, &cloned)
		}
	}
	return reviews, nil
}

func (m *mockReviewRepo) UpdateStatus(_ context.Context, id uuid.UUID, status string) error {
	review, ok := m.byID[id]
	if !ok {
		return service.ErrReviewNotFound
	}
	review.Status = status
	return nil
}

func (m *mockReviewRepo) UpdateResult(_ context.Context, review *model.Review) error {
	cloned := *review
	m.updated = append(m.updated, &cloned)
	m.byID[review.ID] = &cloned
	return nil
}

type mockReviewTaskRepo struct {
	task        *model.Task
	transitions []string
}

func (m *mockReviewTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	if m.task == nil || m.task.ID != id {
		return nil, service.ErrReviewTaskNotFound
	}
	cloned := *m.task
	return &cloned, nil
}

func (m *mockReviewTaskRepo) GetByPRURL(_ context.Context, prURL string) (*model.Task, error) {
	if m.task == nil || m.task.PRUrl != prURL {
		return nil, service.ErrReviewTaskNotFound
	}
	cloned := *m.task
	return &cloned, nil
}

func (m *mockReviewTaskRepo) TransitionStatus(_ context.Context, _ uuid.UUID, newStatus string) error {
	m.transitions = append(m.transitions, newStatus)
	m.task.Status = newStatus
	return nil
}

type mockReviewNotifications struct {
	created []string
}

func (m *mockReviewNotifications) Create(_ context.Context, _ uuid.UUID, ntype, title, body, _ string) (*model.Notification, error) {
	m.created = append(m.created, ntype+"|"+title+"|"+body)
	return &model.Notification{
		ID:        uuid.New(),
		TargetID:  uuid.New(),
		Type:      ntype,
		Title:     title,
		Body:      body,
		CreatedAt: time.Now(),
	}, nil
}

type mockReviewBridge struct {
	response *bridgeclient.ReviewResponse
}

func (m *mockReviewBridge) Review(_ context.Context, _ bridgeclient.ReviewRequest) (*bridgeclient.ReviewResponse, error) {
	return m.response, nil
}

func attachProjectListener(hub *ws.Hub, projectID string) chan []byte {
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

func decodeReviewEvent(t *testing.T, raw []byte) ws.Event {
	t.Helper()

	var event ws.Event
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("unmarshal ws event: %v", err)
	}
	return event
}

func TestReviewService_Trigger_CompletesApproveReview(t *testing.T) {
	taskID := uuid.New()
	assigneeID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:         taskID,
			ProjectID:  uuid.New(),
			Title:      "Implement deep review",
			Status:     model.TaskStatusInProgress,
			AssigneeID: &assigneeID,
			PRUrl:      "https://github.com/acme/project/pull/7",
		},
	}
	reviewRepo := newMockReviewRepo()
	notifications := &mockReviewNotifications{}
	bridge := &mockReviewBridge{
		response: &bridgeclient.ReviewResponse{
			RiskLevel:      model.ReviewRiskLevelMedium,
			Summary:        "Deep review passed with one medium issue already handled",
			Recommendation: model.ReviewRecommendationApprove,
			CostUSD:        0.67,
			Findings: []model.ReviewFinding{
				{Category: "logic", Severity: "medium", Message: "guard clause missing"},
			},
		},
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, notifications, ws.NewHub(), bridge)

	review, err := svc.Trigger(context.Background(), &model.TriggerReviewRequest{
		TaskID:     taskID.String(),
		PRURL:      "https://github.com/acme/project/pull/7",
		PRNumber:   7,
		Trigger:    model.ReviewTriggerAgent,
		Dimensions: []string{"logic", "security"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if review.Status != model.ReviewStatusCompleted {
		t.Fatalf("expected completed review, got %s", review.Status)
	}
	if got := taskRepo.transitions; len(got) != 2 || got[0] != model.TaskStatusInReview || got[1] != model.TaskStatusDone {
		t.Fatalf("expected transitions [in_review done], got %v", got)
	}
	if len(notifications.created) != 1 {
		t.Fatalf("expected one notification, got %d", len(notifications.created))
	}
}

func TestReviewService_Complete_RequestChangesTransitionsTask(t *testing.T) {
	taskID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: uuid.New(),
			Title:     "Review a risky PR",
			Status:    model.TaskStatusInReview,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{
		ID:       reviewID,
		TaskID:   taskID,
		PRURL:    "https://github.com/acme/project/pull/9",
		PRNumber: 9,
		Layer:    2,
		Status:   model.ReviewStatusInProgress,
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil)

	review, err := svc.Complete(context.Background(), reviewID, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelHigh,
		Summary:        "Security review found token leakage",
		Recommendation: model.ReviewRecommendationRequestChanges,
		CostUSD:        1.24,
		Findings: []model.ReviewFinding{
			{Category: "security", Severity: "high", Message: "token leakage"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if review.Recommendation != model.ReviewRecommendationRequestChanges {
		t.Fatalf("expected request_changes recommendation, got %s", review.Recommendation)
	}
	if got := taskRepo.transitions; len(got) != 1 || got[0] != model.TaskStatusChangesRequested {
		t.Fatalf("expected [changes_requested], got %v", got)
	}
}

func TestReviewService_Trigger_DryRunPersistsResultAndBroadcastsEvents(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Deep review dry run",
			Status:    model.TaskStatusInProgress,
			PRUrl:     "https://github.com/acme/project/pull/21",
		},
	}
	reviewRepo := newMockReviewRepo()
	notifications := &mockReviewNotifications{}
	hub := ws.NewHub()
	eventCh := attachProjectListener(hub, projectID.String())
	bridge := &mockReviewBridge{
		response: &bridgeclient.ReviewResponse{
			RiskLevel:      model.ReviewRiskLevelHigh,
			Summary:        "Security and logic issues found during dry run",
			Recommendation: model.ReviewRecommendationRequestChanges,
			CostUSD:        0.91,
			Findings: []model.ReviewFinding{
				{Category: "security", Severity: "high", Message: "Potential secret exposure"},
			},
		},
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, notifications, hub, bridge)

	review, err := svc.Trigger(context.Background(), &model.TriggerReviewRequest{
		TaskID:     taskID.String(),
		PRURL:      "https://github.com/acme/project/pull/21",
		PRNumber:   21,
		Trigger:    model.ReviewTriggerLayer1,
		Dimensions: []string{"logic", "security"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored, ok := reviewRepo.byID[review.ID]
	if !ok {
		t.Fatalf("expected stored review for %s", review.ID)
	}
	if stored.Status != model.ReviewStatusCompleted {
		t.Fatalf("expected stored review status completed, got %s", stored.Status)
	}
	if stored.Recommendation != model.ReviewRecommendationRequestChanges {
		t.Fatalf("expected request_changes recommendation, got %s", stored.Recommendation)
	}
	if len(stored.Findings) != 1 {
		t.Fatalf("expected one persisted finding, got %d", len(stored.Findings))
	}

	created := decodeReviewEvent(t, <-eventCh)
	completed := decodeReviewEvent(t, <-eventCh)

	if created.Type != ws.EventReviewCreated {
		t.Fatalf("expected first event %s, got %s", ws.EventReviewCreated, created.Type)
	}
	if created.ProjectID != projectID.String() {
		t.Fatalf("expected created event project %s, got %s", projectID, created.ProjectID)
	}
	if completed.Type != ws.EventReviewCompleted {
		t.Fatalf("expected second event %s, got %s", ws.EventReviewCompleted, completed.Type)
	}
	if completed.ProjectID != projectID.String() {
		t.Fatalf("expected completed event project %s, got %s", projectID, completed.ProjectID)
	}
}
