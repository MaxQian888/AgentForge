package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
	eventbus "github.com/react-go-quick-starter/server/internal/eventbus"
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

func (m *mockReviewRepo) ListAll(_ context.Context, status, riskLevel string, limit int) ([]*model.Review, error) {
	var reviews []*model.Review
	for _, review := range m.byID {
		if status != "" && review.Status != status {
			continue
		}
		if riskLevel != "" && review.RiskLevel != riskLevel {
			continue
		}
		cloned := *review
		reviews = append(reviews, &cloned)
		if limit > 0 && len(reviews) >= limit {
			break
		}
	}
	return reviews, nil
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

type mockReviewProjectRepo struct {
	projects map[uuid.UUID]*model.Project
}

func (m *mockReviewProjectRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Project, error) {
	project, ok := m.projects[id]
	if !ok {
		return nil, errors.New("project not found")
	}
	cloned := *project
	return &cloned, nil
}

type mockReviewEntityLinkRepo struct {
	links []*model.EntityLink
}

func (m *mockReviewEntityLinkRepo) Create(_ context.Context, link *model.EntityLink) error {
	return nil
}
func (m *mockReviewEntityLinkRepo) GetByID(_ context.Context, id uuid.UUID) (*model.EntityLink, error) {
	for _, link := range m.links {
		if link.ID == id {
			cloned := *link
			return &cloned, nil
		}
	}
	return nil, errors.New("link not found")
}
func (m *mockReviewEntityLinkRepo) Delete(_ context.Context, id uuid.UUID) error { return nil }
func (m *mockReviewEntityLinkRepo) ListBySource(_ context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID) ([]*model.EntityLink, error) {
	result := make([]*model.EntityLink, 0)
	for _, link := range m.links {
		if link.ProjectID == projectID && link.SourceType == sourceType && link.SourceID == sourceID {
			cloned := *link
			result = append(result, &cloned)
		}
	}
	return result, nil
}
func (m *mockReviewEntityLinkRepo) ListByTarget(_ context.Context, projectID uuid.UUID, targetType string, targetID uuid.UUID) ([]*model.EntityLink, error) {
	return nil, nil
}
func (m *mockReviewEntityLinkRepo) UpsertMentionLinks(_ context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID, createdBy uuid.UUID, targets []model.EntityLinkTarget) error {
	return nil
}
func (m *mockReviewEntityLinkRepo) DeleteMentionLinksForSource(_ context.Context, projectID uuid.UUID, sourceType string, sourceID uuid.UUID) error {
	return nil
}

type mockReviewWikiPageRepo struct {
	pages       map[uuid.UUID]*model.WikiPage
	failOnceFor uuid.UUID
	failed      bool
}

func (m *mockReviewWikiPageRepo) Create(_ context.Context, page *model.WikiPage) error { return nil }
func (m *mockReviewWikiPageRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WikiPage, error) {
	page, ok := m.pages[id]
	if !ok {
		return nil, service.ErrWikiPageNotFound
	}
	cloned := *page
	return &cloned, nil
}
func (m *mockReviewWikiPageRepo) Update(_ context.Context, page *model.WikiPage) error {
	if m.failOnceFor == page.ID && !m.failed {
		m.failed = true
		return service.ErrWikiPageConflict
	}
	cloned := *page
	m.pages[page.ID] = &cloned
	return nil
}
func (m *mockReviewWikiPageRepo) SoftDelete(_ context.Context, id uuid.UUID) error { return nil }
func (m *mockReviewWikiPageRepo) ListTree(_ context.Context, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	return nil, nil
}
func (m *mockReviewWikiPageRepo) ListByParent(_ context.Context, spaceID uuid.UUID, parentID *uuid.UUID) ([]*model.WikiPage, error) {
	return nil, nil
}
func (m *mockReviewWikiPageRepo) MovePage(_ context.Context, id uuid.UUID, parentID *uuid.UUID, path string, sortOrder int) error {
	return nil
}
func (m *mockReviewWikiPageRepo) UpdateSortOrder(_ context.Context, id uuid.UUID, sortOrder int) error {
	return nil
}

type mockReviewPageVersionRepo struct {
	versions []*model.PageVersion
}

func (m *mockReviewPageVersionRepo) Create(_ context.Context, version *model.PageVersion) error {
	cloned := *version
	m.versions = append(m.versions, &cloned)
	return nil
}
func (m *mockReviewPageVersionRepo) ListByPageID(_ context.Context, pageID uuid.UUID) ([]*model.PageVersion, error) {
	result := make([]*model.PageVersion, 0)
	for _, version := range m.versions {
		if version.PageID == pageID {
			cloned := *version
			result = append(result, &cloned)
		}
	}
	return result, nil
}
func (m *mockReviewPageVersionRepo) GetByID(_ context.Context, id uuid.UUID) (*model.PageVersion, error) {
	return nil, nil
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

type reviewEventCapture struct {
	Type      string
	ProjectID string
	Payload   any
}

type reviewBusCapture struct {
	mu     sync.Mutex
	events []reviewEventCapture
}

func (c *reviewBusCapture) Publish(_ context.Context, e *eventbus.Event) error {
	var payload any
	if len(e.Payload) > 0 {
		_ = json.Unmarshal(e.Payload, &payload)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, reviewEventCapture{
		Type:      e.Type,
		ProjectID: eventbus.GetString(e, eventbus.MetaProjectID),
		Payload:   payload,
	})
	return nil
}

func (c *reviewBusCapture) next(t *testing.T) reviewEventCapture {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) == 0 {
		t.Fatal("expected at least one event, got none")
	}
	head := c.events[0]
	c.events = c.events[1:]
	return head
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

	svc := service.NewReviewService(reviewRepo, taskRepo, notifications, ws.NewHub(), nil, bridge)

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

func TestReviewService_CompleteEmitsAutomationEvent(t *testing.T) {
	taskID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: uuid.New(),
			Title:     "Automation review",
			Status:    model.TaskStatusInReview,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{
		ID:     reviewID,
		TaskID: taskID,
		Status: model.ReviewStatusInProgress,
	}
	automation := &automationEventProbe{}
	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil)
	svc.SetAutomationEvaluator(automation)

	if _, err := svc.Complete(context.Background(), reviewID, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelLow,
		Summary:        "ok",
		Recommendation: model.ReviewRecommendationApprove,
	}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if len(automation.events) != 1 || automation.events[0].EventType != model.AutomationEventReviewCompleted {
		t.Fatalf("automation events = %+v", automation.events)
	}
}

type automationEventProbe struct {
	events []service.AutomationEvent
}

func (p *automationEventProbe) EvaluateRules(_ context.Context, event service.AutomationEvent) error {
	p.events = append(p.events, event)
	return nil
}

type automationIMBridge struct{ sent *model.IMSendRequest }

func (b *automationIMBridge) Send(_ context.Context, req *model.IMSendRequest) error {
	b.sent = req
	return nil
}

type automationRuleRepoService struct{ rules []*model.AutomationRule }

func (r *automationRuleRepoService) ListByProjectAndEvent(_ context.Context, projectID uuid.UUID, eventType string) ([]*model.AutomationRule, error) {
	result := make([]*model.AutomationRule, 0)
	for _, rule := range r.rules {
		if rule.ProjectID == projectID && rule.EventType == eventType {
			result = append(result, rule)
		}
	}
	return result, nil
}

type automationLogRepoService struct{ entries []*model.AutomationLog }

func (r *automationLogRepoService) Create(_ context.Context, entry *model.AutomationLog) error {
	r.entries = append(r.entries, entry)
	return nil
}

func TestReviewService_CompleteRunsAutomationIMFlow(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Review IM flow",
			Status:    model.TaskStatusInReview,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{ID: reviewID, TaskID: taskID, Status: model.ReviewStatusInProgress}
	im := &automationIMBridge{}
	logs := &automationLogRepoService{}
	engine := service.NewAutomationEngineService(
		&automationRuleRepoService{rules: []*model.AutomationRule{{
			ID:         uuid.New(),
			ProjectID:  projectID,
			Enabled:    true,
			EventType:  model.AutomationEventReviewCompleted,
			Conditions: `[]`,
			Actions:    `[{"type":"send_im_message","config":{"platform":"slack","channelId":"C1","text":"review completed"}}]`,
		}}},
		logs,
		nil,
		nil,
		nil,
		im,
		nil,
	)
	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil)
	svc.SetAutomationEvaluator(engine)

	if _, err := svc.Complete(context.Background(), reviewID, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelLow,
		Summary:        "ok",
		Recommendation: model.ReviewRecommendationApprove,
	}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if im.sent == nil || im.sent.ChannelID != "C1" {
		t.Fatalf("im send = %+v", im.sent)
	}
	if len(logs.entries) != 1 || logs.entries[0].Status != model.AutomationLogStatusSuccess {
		t.Fatalf("automation logs = %+v", logs.entries)
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

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil)

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

func TestReviewService_CompleteWritesBackToRequirementDoc(t *testing.T) {
	taskID := uuid.New()
	reviewID := uuid.New()
	pageID := uuid.New()
	projectID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{ID: taskID, ProjectID: projectID, Title: "Writeback", Status: model.TaskStatusInReview},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{ID: reviewID, TaskID: taskID, Status: model.ReviewStatusInProgress}
	pageRepo := &mockReviewWikiPageRepo{
		pages: map[uuid.UUID]*model.WikiPage{
			pageID: {ID: pageID, SpaceID: uuid.New(), Title: "PRD", Content: `[{"id":"a","type":"paragraph","content":"before"}]`, UpdatedAt: time.Now().UTC()},
		},
	}
	versionRepo := &mockReviewPageVersionRepo{}
	linkRepo := &mockReviewEntityLinkRepo{
		links: []*model.EntityLink{{
			ID:         uuid.New(),
			ProjectID:  projectID,
			SourceType: model.EntityTypeTask,
			SourceID:   taskID,
			TargetType: model.EntityTypeWikiPage,
			TargetID:   pageID,
			LinkType:   model.EntityLinkTypeRequirement,
		}},
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil).
		WithDocWriteback(linkRepo, pageRepo, versionRepo)

	review, err := svc.Complete(context.Background(), reviewID, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelHigh,
		Summary:        "Summary",
		Recommendation: model.ReviewRecommendationRequestChanges,
		Findings:       []model.ReviewFinding{{Severity: "high", Message: "Issue"}},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if review == nil {
		t.Fatal("expected review")
	}
	if len(versionRepo.versions) != 1 {
		t.Fatalf("len(versions) = %d, want 1", len(versionRepo.versions))
	}
	if got := pageRepo.pages[pageID].Content; !strings.Contains(got, "Review Findings") || !strings.Contains(got, "Issue") {
		t.Fatalf("page content = %q", got)
	}
}

func TestReviewService_CompleteSkipsWritebackWithoutLinkedDoc(t *testing.T) {
	taskID := uuid.New()
	reviewID := uuid.New()
	projectID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{ID: taskID, ProjectID: projectID, Title: "No doc", Status: model.TaskStatusInReview},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{ID: reviewID, TaskID: taskID, Status: model.ReviewStatusInProgress}
	versionRepo := &mockReviewPageVersionRepo{}
	linkRepo := &mockReviewEntityLinkRepo{}
	pageRepo := &mockReviewWikiPageRepo{pages: map[uuid.UUID]*model.WikiPage{}}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil).
		WithDocWriteback(linkRepo, pageRepo, versionRepo)

	if _, err := svc.Complete(context.Background(), reviewID, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelLow,
		Summary:        "ok",
		Recommendation: model.ReviewRecommendationApprove,
	}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if len(versionRepo.versions) != 0 {
		t.Fatalf("len(versions) = %d, want 0", len(versionRepo.versions))
	}
}

func TestReviewService_CompleteRetriesWritebackOnConflict(t *testing.T) {
	taskID := uuid.New()
	reviewID := uuid.New()
	pageID := uuid.New()
	projectID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{ID: taskID, ProjectID: projectID, Title: "Retry", Status: model.TaskStatusInReview},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{ID: reviewID, TaskID: taskID, Status: model.ReviewStatusInProgress}
	pageRepo := &mockReviewWikiPageRepo{
		pages: map[uuid.UUID]*model.WikiPage{
			pageID: {ID: pageID, SpaceID: uuid.New(), Title: "ADR", Content: `[{"id":"a","type":"paragraph","content":"before"}]`, UpdatedAt: time.Now().UTC()},
		},
		failOnceFor: pageID,
	}
	versionRepo := &mockReviewPageVersionRepo{}
	linkRepo := &mockReviewEntityLinkRepo{
		links: []*model.EntityLink{{
			ID:         uuid.New(),
			ProjectID:  projectID,
			SourceType: model.EntityTypeTask,
			SourceID:   taskID,
			TargetType: model.EntityTypeWikiPage,
			TargetID:   pageID,
			LinkType:   model.EntityLinkTypeDesign,
		}},
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil).
		WithDocWriteback(linkRepo, pageRepo, versionRepo)

	if _, err := svc.Complete(context.Background(), reviewID, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelMedium,
		Summary:        "retry",
		Recommendation: model.ReviewRecommendationRequestChanges,
		Findings:       []model.ReviewFinding{{Severity: "medium", Message: "Retry issue"}},
	}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if !pageRepo.failed {
		t.Fatal("expected first writeback update to fail once")
	}
	if got := pageRepo.pages[pageID].Content; !strings.Contains(got, "Retry issue") {
		t.Fatalf("page content = %q", got)
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
	capture := &reviewBusCapture{}
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

	svc := service.NewReviewService(reviewRepo, taskRepo, notifications, ws.NewHub(), capture, bridge)

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

	created := capture.next(t)
	completed := capture.next(t)

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

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, bridge).
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

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, bridge).
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
	capture := &reviewBusCapture{}
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

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), capture, bridge).
		WithExecutionPlanner(planner)

	if _, err := svc.Trigger(context.Background(), &model.TriggerReviewRequest{
		TaskID:   taskID.String(),
		PRURL:    "https://github.com/acme/project/pull/92",
		PRNumber: 92,
		Trigger:  model.ReviewTriggerManual,
	}); err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}

	_ = capture.next(t)
	completed := capture.next(t)
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

func TestReviewService_CompleteRoutesToPendingHumanWhenManualApprovalRequired(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Policy gated review",
			Status:    model.TaskStatusInReview,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{
		ID:       reviewID,
		TaskID:   taskID,
		Status:   model.ReviewStatusInProgress,
		RiskLevel: model.ReviewRiskLevelLow,
	}
	projectRepo := &mockReviewProjectRepo{
		projects: map[uuid.UUID]*model.Project{
			projectID: {
				ID: projectID,
				Settings: `{"review_policy":{"requiredLayers":["layer2"],"requireManualApproval":true,"minRiskLevelForBlock":""}}`,
			},
		},
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil).
		WithProjectRepository(projectRepo)

	review, err := svc.Complete(context.Background(), reviewID, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelLow,
		Summary:        "automation completed",
		Recommendation: model.ReviewRecommendationApprove,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if review.Status != model.ReviewStatusPendingHuman {
		t.Fatalf("status = %s, want pending_human", review.Status)
	}
	if len(taskRepo.transitions) != 0 {
		t.Fatalf("transitions = %#v, want none before human approval", taskRepo.transitions)
	}
}

func TestReviewService_CompleteRoutesToPendingHumanWhenRiskThresholdExceeded(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Threshold review",
			Status:    model.TaskStatusInReview,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{
		ID:       reviewID,
		TaskID:   taskID,
		Status:   model.ReviewStatusInProgress,
		RiskLevel: model.ReviewRiskLevelLow,
	}
	projectRepo := &mockReviewProjectRepo{
		projects: map[uuid.UUID]*model.Project{
			projectID: {
				ID: projectID,
				Settings: `{"review_policy":{"requiredLayers":[],"requireManualApproval":false,"minRiskLevelForBlock":"high"}}`,
			},
		},
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil).
		WithProjectRepository(projectRepo)

	review, err := svc.Complete(context.Background(), reviewID, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelMedium,
		Summary:        "high-risk finding detected",
		Recommendation: model.ReviewRecommendationApprove,
		Findings: []model.ReviewFinding{
			{ID: "finding-1", Category: "security", Severity: model.ReviewRiskLevelHigh, Message: "secret leakage"},
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if review.Status != model.ReviewStatusPendingHuman {
		t.Fatalf("status = %s, want pending_human", review.Status)
	}
}

func TestReviewService_CompleteAutoResolvesWhenPolicyPasses(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Auto resolve review",
			Status:    model.TaskStatusInReview,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{
		ID:       reviewID,
		TaskID:   taskID,
		Status:   model.ReviewStatusInProgress,
		RiskLevel: model.ReviewRiskLevelLow,
	}
	projectRepo := &mockReviewProjectRepo{
		projects: map[uuid.UUID]*model.Project{
			projectID: {
				ID: projectID,
				Settings: `{"review_policy":{"requiredLayers":[],"requireManualApproval":false,"minRiskLevelForBlock":"critical"}}`,
			},
		},
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil).
		WithProjectRepository(projectRepo)

	review, err := svc.Complete(context.Background(), reviewID, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelLow,
		Summary:        "safe review",
		Recommendation: model.ReviewRecommendationApprove,
		Findings: []model.ReviewFinding{
			{ID: "finding-1", Category: "style", Severity: model.ReviewRiskLevelLow, Message: "minor issue"},
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if review.Status != model.ReviewStatusCompleted {
		t.Fatalf("status = %s, want completed", review.Status)
	}
	if len(taskRepo.transitions) != 1 || taskRepo.transitions[0] != model.TaskStatusDone {
		t.Fatalf("transitions = %#v, want [done]", taskRepo.transitions)
	}
}

func TestReviewService_ApproveReviewPreservesEvidenceAndAppendsDecision(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Human approval",
			Status:    model.TaskStatusInReview,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{
		ID:       reviewID,
		TaskID:   taskID,
		Status:   model.ReviewStatusPendingHuman,
		RiskLevel: model.ReviewRiskLevelHigh,
		Findings: []model.ReviewFinding{
			{ID: "finding-1", Category: "security", Severity: "high", Message: "token exposure"},
		},
		ExecutionMetadata: &model.ReviewExecutionMetadata{
			TriggerEvent: "pull_request.updated",
		},
		Summary:        "automation summary",
		Recommendation: model.ReviewRecommendationRequestChanges,
		CostUSD:        1.2,
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil)

	review, err := svc.ApproveReview(context.Background(), reviewID, "reviewer-1", "Looks good")
	if err != nil {
		t.Fatalf("ApproveReview() error = %v", err)
	}

	if review.Status != model.ReviewStatusCompleted {
		t.Fatalf("status = %s, want completed", review.Status)
	}
	if review.Recommendation != model.ReviewRecommendationApprove {
		t.Fatalf("recommendation = %s, want approve", review.Recommendation)
	}
	if review.Summary != "automation summary" {
		t.Fatalf("summary = %q, evidence must be preserved", review.Summary)
	}
	if review.CostUSD != 1.2 {
		t.Fatalf("cost = %f, evidence must be preserved", review.CostUSD)
	}
	if len(review.ExecutionMetadata.Decisions) != 1 {
		t.Fatalf("decisions = %#v, want 1", review.ExecutionMetadata.Decisions)
	}
	if review.ExecutionMetadata.Decisions[0].Actor != "reviewer-1" {
		t.Fatalf("actor = %q", review.ExecutionMetadata.Decisions[0].Actor)
	}
}

func TestReviewService_RequestChangesReviewPreservesEvidenceAndAppendsDecision(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Human request changes",
			Status:    model.TaskStatusInReview,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{
		ID:       reviewID,
		TaskID:   taskID,
		Status:   model.ReviewStatusPendingHuman,
		RiskLevel: model.ReviewRiskLevelHigh,
		Findings: []model.ReviewFinding{
			{ID: "finding-1", Category: "security", Severity: "high", Message: "token exposure"},
		},
		ExecutionMetadata: &model.ReviewExecutionMetadata{
			TriggerEvent: "pull_request.updated",
		},
		Summary:        "automation summary",
		Recommendation: model.ReviewRecommendationApprove,
		CostUSD:        1.2,
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil)

	review, err := svc.RequestChangesReview(context.Background(), reviewID, "reviewer-2", "Needs hardening")
	if err != nil {
		t.Fatalf("RequestChangesReview() error = %v", err)
	}

	if review.Status != model.ReviewStatusCompleted {
		t.Fatalf("status = %s, want completed", review.Status)
	}
	if review.Recommendation != model.ReviewRecommendationRequestChanges {
		t.Fatalf("recommendation = %s, want request_changes", review.Recommendation)
	}
	if review.Summary != "automation summary" {
		t.Fatalf("summary = %q, evidence must be preserved", review.Summary)
	}
	if review.CostUSD != 1.2 {
		t.Fatalf("cost = %f, evidence must be preserved", review.CostUSD)
	}
	if len(review.ExecutionMetadata.Decisions) != 1 {
		t.Fatalf("decisions = %#v, want 1", review.ExecutionMetadata.Decisions)
	}
	if len(taskRepo.transitions) != 1 || taskRepo.transitions[0] != model.TaskStatusChangesRequested {
		t.Fatalf("transitions = %#v, want [changes_requested]", taskRepo.transitions)
	}
}

func TestReviewService_RejectReviewPreservesEvidenceAndAppendsDecision(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Human reject",
			Status:    model.TaskStatusInReview,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{
		ID:       reviewID,
		TaskID:   taskID,
		Status:   model.ReviewStatusPendingHuman,
		RiskLevel: model.ReviewRiskLevelCritical,
		Findings: []model.ReviewFinding{
			{ID: "finding-1", Category: "security", Severity: "critical", Message: "critical vulnerability"},
		},
		ExecutionMetadata: &model.ReviewExecutionMetadata{
			TriggerEvent: "pull_request.updated",
		},
		Summary:        "automation summary",
		Recommendation: model.ReviewRecommendationRequestChanges,
		CostUSD:        2.1,
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil)

	review, err := svc.RejectReview(context.Background(), reviewID, "reviewer-3", "Security risk", "must close")
	if err != nil {
		t.Fatalf("RejectReview() error = %v", err)
	}

	if review.Status != model.ReviewStatusFailed {
		t.Fatalf("status = %s, want failed", review.Status)
	}
	if review.Recommendation != model.ReviewRecommendationReject {
		t.Fatalf("recommendation = %s, want reject", review.Recommendation)
	}
	if review.Summary != "automation summary" {
		t.Fatalf("summary = %q, evidence must be preserved", review.Summary)
	}
	if len(review.ExecutionMetadata.Decisions) != 1 {
		t.Fatalf("decisions = %#v, want 1", review.ExecutionMetadata.Decisions)
	}
	if len(taskRepo.transitions) != 1 || taskRepo.transitions[0] != model.TaskStatusCancelled {
		t.Fatalf("transitions = %#v, want [cancelled]", taskRepo.transitions)
	}
}

func TestReviewService_MarkFalsePositiveDismissesMatchedFinding(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	reviewID := uuid.New()
	taskRepo := &mockReviewTaskRepo{
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "False positive review",
			Status:    model.TaskStatusDone,
		},
	}
	reviewRepo := newMockReviewRepo()
	reviewRepo.byID[reviewID] = &model.Review{
		ID:       reviewID,
		TaskID:   taskID,
		Status:   model.ReviewStatusCompleted,
		RiskLevel: model.ReviewRiskLevelMedium,
		Findings: []model.ReviewFinding{
			{ID: "finding-1", Category: "security", Severity: "medium", Message: "allowed endpoint"},
			{ID: "finding-2", Category: "style", Severity: "low", Message: "formatting"},
		},
		ExecutionMetadata: &model.ReviewExecutionMetadata{},
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, nil)

	review, err := svc.MarkFalsePositive(context.Background(), reviewID, "reviewer-2", []string{"finding-1"}, "known acceptable risk")
	if err != nil {
		t.Fatalf("MarkFalsePositive() error = %v", err)
	}

	if review.Status != model.ReviewStatusCompleted {
		t.Fatalf("status = %s, want completed", review.Status)
	}
	if !review.Findings[0].Dismissed {
		t.Fatalf("finding[0] dismissed = %v, want true", review.Findings[0].Dismissed)
	}
	if review.Findings[1].Dismissed {
		t.Fatalf("finding[1] dismissed = %v, want false", review.Findings[1].Dismissed)
	}
	if len(review.ExecutionMetadata.Decisions) != 1 {
		t.Fatalf("decisions = %#v, want 1", review.ExecutionMetadata.Decisions)
	}
}

func TestReviewService_TriggerAllowsStandaloneDeepReview(t *testing.T) {
	projectID := uuid.New()
	reviewRepo := newMockReviewRepo()
	taskRepo := &mockReviewTaskRepo{}
	bridge := &mockReviewBridge{
		response: &bridgeclient.ReviewResponse{
			RiskLevel:      model.ReviewRiskLevelLow,
			Summary:        "standalone review complete",
			Recommendation: model.ReviewRecommendationApprove,
			CostUSD:        0.2,
			Findings:       []model.ReviewFinding{},
		},
	}

	svc := service.NewReviewService(reviewRepo, taskRepo, &mockReviewNotifications{}, ws.NewHub(), nil, bridge)

	review, err := svc.Trigger(context.Background(), &model.TriggerReviewRequest{
		TaskID:    "",
		ProjectID: projectID.String(),
		PRURL:     "https://github.com/acme/project/pull/123",
		PRNumber:  123,
		Trigger:   model.ReviewTriggerManual,
	})
	if err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}

	if review.TaskID != uuid.Nil {
		t.Fatalf("TaskID = %s, want nil UUID for detached review", review.TaskID)
	}
	if review.Status != model.ReviewStatusCompleted {
		t.Fatalf("status = %s, want completed", review.Status)
	}
	if review.ExecutionMetadata == nil || review.ExecutionMetadata.ProjectID != projectID.String() {
		t.Fatalf("execution metadata project = %+v, want %s", review.ExecutionMetadata, projectID)
	}
}
