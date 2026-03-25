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
	req      *bridgeclient.ReviewRequest
}

func (m *mockReviewBridge) Review(_ context.Context, req bridgeclient.ReviewRequest) (*bridgeclient.ReviewResponse, error) {
	cloned := req
	cloned.Dimensions = append([]string(nil), req.Dimensions...)
	cloned.ChangedFiles = append([]string(nil), req.ChangedFiles...)
	cloned.ReviewPlugins = append([]bridgeclient.ReviewPluginRequest(nil), req.ReviewPlugins...)
	m.req = &cloned
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

func TestReviewService_Trigger_ForwardsMatchingReviewPluginPlanToBridge(t *testing.T) {
	taskID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: uuid.New(),
			Title:     "Review plugin selection",
			Status:    model.TaskStatusInProgress,
			PRUrl:     "https://github.com/acme/project/pull/88",
		},
	}
	reviewRepo := newMockReviewRepo()
	bridge := &mockReviewBridge{
		response: &bridgeclient.ReviewResponse{
			RiskLevel:      model.ReviewRiskLevelLow,
			Summary:        "Deep review completed",
			Recommendation: model.ReviewRecommendationApprove,
			CostUSD:        0.12,
		},
	}

	planner := service.NewReviewExecutionPlanner(reviewPluginCatalogStub{
		records: []*model.PluginRecord{
			{
				PluginManifest: model.PluginManifest{
					Kind: model.PluginKindReview,
					Metadata: model.PluginMetadata{
						ID:   "review.typescript",
						Name: "TypeScript Review",
					},
					Spec: model.PluginSpec{
						Runtime: model.PluginRuntimeMCP,
						Review: &model.ReviewPluginSpec{
							Entrypoint: "review:run",
							Triggers: model.ReviewPluginTrigger{
								Events:       []string{"pull_request.updated"},
								FilePatterns: []string{"src/**/*.ts"},
							},
							Output: model.ReviewPluginOutput{Format: "findings/v1"},
						},
					},
					Source: model.PluginSource{Type: model.PluginSourceNPM},
				},
				LifecycleState: model.PluginStateEnabled,
			},
		},
	})

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), bridge).
		WithExecutionPlanner(planner)

	_, err := svc.Trigger(context.Background(), &model.TriggerReviewRequest{
		TaskID:   taskID.String(),
		PRURL:    "https://github.com/acme/project/pull/88",
		PRNumber: 88,
		Trigger:  model.ReviewTriggerManual,
		Diff: "diff --git a/src/plugins/runtime.ts b/src/plugins/runtime.ts\n" +
			"index 111..222 100644\n" +
			"--- a/src/plugins/runtime.ts\n" +
			"+++ b/src/plugins/runtime.ts\n",
	})
	if err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}

	if bridge.req == nil {
		t.Fatal("expected bridge request to be captured")
	}
	if bridge.req.TriggerEvent != "pull_request.updated" {
		t.Fatalf("TriggerEvent = %q, want pull_request.updated", bridge.req.TriggerEvent)
	}
	if len(bridge.req.ChangedFiles) != 1 || bridge.req.ChangedFiles[0] != "src/plugins/runtime.ts" {
		t.Fatalf("ChangedFiles = %#v, want [src/plugins/runtime.ts]", bridge.req.ChangedFiles)
	}
	if len(bridge.req.ReviewPlugins) != 1 || bridge.req.ReviewPlugins[0].PluginID != "review.typescript" {
		t.Fatalf("ReviewPlugins = %#v, want review.typescript", bridge.req.ReviewPlugins)
	}
	if bridge.req.ReviewPlugins[0].OutputFormat != "findings/v1" {
		t.Fatalf("OutputFormat = %q, want findings/v1", bridge.req.ReviewPlugins[0].OutputFormat)
	}
}

func TestReviewService_Trigger_PersistsExecutionMetadataFromBridgeResponse(t *testing.T) {
	taskID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: uuid.New(),
			Title:     "Deep review metadata",
			Status:    model.TaskStatusInProgress,
			PRUrl:     "https://github.com/acme/project/pull/91",
		},
	}
	reviewRepo := newMockReviewRepo()
	bridge := &mockReviewBridge{
		response: &bridgeclient.ReviewResponse{
			RiskLevel:      model.ReviewRiskLevelHigh,
			Summary:        "plugin-aware review completed",
			Recommendation: model.ReviewRecommendationRequestChanges,
			CostUSD:        0.23,
			Findings: []model.ReviewFinding{
				{Category: "security", Severity: "high", Message: "Potential secret exposure", Sources: []string{"security"}},
				{Category: "architecture", Severity: "medium", Message: "Architecture drift", Sources: []string{"review.architecture"}},
			},
			DimensionResults: []bridgeclient.ReviewExecutionResult{
				{Dimension: "security", SourceType: "builtin", Status: "completed", Summary: "Security found one issue"},
				{Dimension: "review.architecture", SourceType: "plugin", PluginID: "review.architecture", DisplayName: "Architecture Review", Status: "failed", Summary: "Architecture plugin failed", Error: "timeout"},
			},
		},
	}

	planner := service.NewReviewExecutionPlanner(reviewPluginCatalogStub{
		records: []*model.PluginRecord{
			{
				PluginManifest: model.PluginManifest{
					Kind:     model.PluginKindReview,
					Metadata: model.PluginMetadata{ID: "review.architecture", Name: "Architecture Review"},
					Spec: model.PluginSpec{
						Runtime: model.PluginRuntimeMCP,
						Review: &model.ReviewPluginSpec{
							Entrypoint: "review:run",
							Triggers: model.ReviewPluginTrigger{
								Events:       []string{"pull_request.updated"},
								FilePatterns: []string{"src/**/*.go"},
							},
							Output: model.ReviewPluginOutput{Format: "findings/v1"},
						},
					},
					Source: model.PluginSource{Type: model.PluginSourceLocal},
				},
				LifecycleState: model.PluginStateEnabled,
			},
		},
	})

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), bridge).
		WithExecutionPlanner(planner)

	review, err := svc.Trigger(context.Background(), &model.TriggerReviewRequest{
		TaskID:   taskID.String(),
		PRURL:    "https://github.com/acme/project/pull/91",
		PRNumber: 91,
		Trigger:  model.ReviewTriggerManual,
		Diff: "diff --git a/src/server/routes.go b/src/server/routes.go\n" +
			"index 111..222 100644\n" +
			"--- a/src/server/routes.go\n" +
			"+++ b/src/server/routes.go\n",
	})
	if err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}

	if review.ExecutionMetadata == nil {
		t.Fatal("expected execution metadata on completed review")
	}
	if review.ExecutionMetadata.TriggerEvent != "pull_request.updated" {
		t.Fatalf("TriggerEvent = %q, want pull_request.updated", review.ExecutionMetadata.TriggerEvent)
	}
	if len(review.ExecutionMetadata.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(review.ExecutionMetadata.Results))
	}
	if review.ExecutionMetadata.Results[1].Kind != model.ReviewExecutionKindPlugin {
		t.Fatalf("Results[1].Kind = %q, want %q", review.ExecutionMetadata.Results[1].Kind, model.ReviewExecutionKindPlugin)
	}
	dto := review.ToDTO()
	if dto.ExecutionMetadata == nil || len(dto.ExecutionMetadata.Results) != 2 {
		t.Fatalf("expected execution metadata in DTO, got %+v", dto.ExecutionMetadata)
	}
}

func TestReviewService_Trigger_BroadcastsExecutionMetadataInCompletedEvent(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Review websocket metadata",
			Status:    model.TaskStatusInProgress,
			PRUrl:     "https://github.com/acme/project/pull/92",
		},
	}
	reviewRepo := newMockReviewRepo()
	hub := ws.NewHub()
	eventCh := attachProjectListener(hub, projectID.String())
	bridge := &mockReviewBridge{
		response: &bridgeclient.ReviewResponse{
			RiskLevel:      model.ReviewRiskLevelLow,
			Summary:        "completed with plugin metadata",
			Recommendation: model.ReviewRecommendationApprove,
			CostUSD:        0.1,
			DimensionResults: []bridgeclient.ReviewExecutionResult{
				{Dimension: "security", SourceType: "builtin", Status: "completed", Summary: "Security ok"},
				{Dimension: "review.architecture", SourceType: "plugin", PluginID: "review.architecture", Status: "failed", Summary: "Architecture plugin failed", Error: "timeout"},
			},
		},
	}
	planner := service.NewReviewExecutionPlanner(reviewPluginCatalogStub{
		records: []*model.PluginRecord{
			{
				PluginManifest: model.PluginManifest{
					Kind:     model.PluginKindReview,
					Metadata: model.PluginMetadata{ID: "review.architecture", Name: "Architecture Review"},
					Spec: model.PluginSpec{
						Runtime: "mcp",
						Review: &model.ReviewPluginSpec{
							Entrypoint: "review:run",
							Triggers:   model.ReviewPluginTrigger{Events: []string{"pull_request.updated"}},
							Output:     model.ReviewPluginOutput{Format: "findings/v1"},
						},
					},
				},
				LifecycleState: model.PluginStateEnabled,
			},
		},
	})

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, hub, bridge).
		WithExecutionPlanner(planner)

	if _, err := svc.Trigger(context.Background(), &model.TriggerReviewRequest{
		TaskID:   taskID.String(),
		PRURL:    "https://github.com/acme/project/pull/92",
		PRNumber: 92,
		Trigger:  model.ReviewTriggerManual,
	}); err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}

	_ = decodeReviewEvent(t, <-eventCh)
	completed := decodeReviewEvent(t, <-eventCh)
	payload, ok := completed.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload map, got %#v", completed.Payload)
	}
	executionMetadata, ok := payload["executionMetadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected executionMetadata in websocket payload, got %#v", payload["executionMetadata"])
	}
	results, ok := executionMetadata["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("expected 2 execution results in websocket payload, got %#v", executionMetadata["results"])
	}
}
