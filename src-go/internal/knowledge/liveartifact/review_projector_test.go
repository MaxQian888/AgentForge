package liveartifact

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type stubReviewReader struct {
	review *model.Review
	err    error
}

func (s *stubReviewReader) GetByID(_ context.Context, _ uuid.UUID) (*model.Review, error) {
	return s.review, s.err
}

type stubTaskReader struct {
	task *model.Task
	err  error
}

func (s *stubTaskReader) GetByID(_ context.Context, _ uuid.UUID) (*model.Task, error) {
	return s.task, s.err
}

func reviewRef(id uuid.UUID) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"kind": "review", "id": id.String()})
	return b
}

func newReviewFixture(projectID uuid.UUID, status string) (*model.Review, *model.Task) {
	reviewID := uuid.New()
	taskID := uuid.New()
	task := &model.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "My Task",
	}
	review := &model.Review{
		ID:             reviewID,
		TaskID:         taskID,
		Layer:          2,
		Status:         status,
		RiskLevel:      model.ReviewRiskLevelMedium,
		Recommendation: model.ReviewRecommendationApprove,
		CostUSD:        0.25,
		UpdatedAt:      time.Now(),
	}
	return review, task
}

func TestReviewProjectorInProgress(t *testing.T) {
	projectID := uuid.New()
	review, task := newReviewFixture(projectID, model.ReviewStatusInProgress)
	p := NewReviewProjector(&stubReviewReader{review: review}, &stubTaskReader{task: task})

	res, err := p.Project(context.Background(), viewerPrincipal(), projectID, reviewRef(review.ID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s (diag=%s)", res.Status, res.Diagnostics)
	}
	text := flattenBlocks(t, res.Projection)
	if !strings.Contains(text, "Status: in_progress") {
		t.Fatalf("projection missing in_progress status: %s", text)
	}
	if res.TTLHint == nil || *res.TTLHint != 30*time.Second {
		t.Fatalf("want 30s TTL hint, got %v", res.TTLHint)
	}
}

func TestReviewProjectorCompletedWithFindings(t *testing.T) {
	projectID := uuid.New()
	review, task := newReviewFixture(projectID, model.ReviewStatusCompleted)
	review.Findings = []model.ReviewFinding{
		{Category: "security", Severity: model.ReviewRiskLevelCritical, Message: "sqli"},
		{Category: "style", Severity: model.ReviewRiskLevelLow, Message: "nit"},
	}
	p := NewReviewProjector(&stubReviewReader{review: review}, &stubTaskReader{task: task})

	res, err := p.Project(context.Background(), viewerPrincipal(), projectID, reviewRef(review.ID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	text := flattenBlocks(t, res.Projection)
	if !strings.Contains(text, "Findings: 2 total (1 critical / 0 high / 0 medium / 1 low)") {
		t.Fatalf("projection missing findings breakdown: %s", text)
	}
}

func TestReviewProjectorPendingHuman(t *testing.T) {
	projectID := uuid.New()
	review, task := newReviewFixture(projectID, model.ReviewStatusPendingHuman)
	p := NewReviewProjector(&stubReviewReader{review: review}, &stubTaskReader{task: task})

	res, err := p.Project(context.Background(), viewerPrincipal(), projectID, reviewRef(review.ID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	text := flattenBlocks(t, res.Projection)
	if !strings.Contains(text, "Status: pending_human") {
		t.Fatalf("projection missing pending_human status: %s", text)
	}
}

func TestReviewProjectorLinkedTaskTitle(t *testing.T) {
	projectID := uuid.New()
	review, task := newReviewFixture(projectID, model.ReviewStatusCompleted)
	task.Title = "My Task"
	p := NewReviewProjector(&stubReviewReader{review: review}, &stubTaskReader{task: task})

	res, err := p.Project(context.Background(), viewerPrincipal(), projectID, reviewRef(review.ID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	text := flattenBlocks(t, res.Projection)
	if !strings.Contains(text, "Review: My Task") {
		t.Fatalf("expected heading 'Review: My Task', got: %s", text)
	}
}

func TestReviewProjectorLinkedTaskFallback(t *testing.T) {
	projectID := uuid.New()
	review, _ := newReviewFixture(projectID, model.ReviewStatusCompleted)
	p := NewReviewProjector(
		&stubReviewReader{review: review},
		&stubTaskReader{task: nil, err: gorm.ErrRecordNotFound},
	)

	res, err := p.Project(context.Background(), viewerPrincipal(), projectID, reviewRef(review.ID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK on missing task, got %s (diag=%s)", res.Status, res.Diagnostics)
	}
	text := flattenBlocks(t, res.Projection)
	want := "Review: Task " + review.TaskID.String()
	if !strings.Contains(text, want) {
		t.Fatalf("expected fallback heading %q, got: %s", want, text)
	}
}

func TestReviewProjectorCrossProjectMismatch(t *testing.T) {
	askedProject := uuid.New()
	otherProject := uuid.New()
	review, task := newReviewFixture(otherProject, model.ReviewStatusCompleted)
	p := NewReviewProjector(&stubReviewReader{review: review}, &stubTaskReader{task: task})

	res, err := p.Project(context.Background(), viewerPrincipal(), askedProject, reviewRef(review.ID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusNotFound {
		t.Fatalf("want StatusNotFound on cross-project, got %s", res.Status)
	}
	if len(res.Projection) != 0 {
		t.Fatalf("cross-project result must not leak projection JSON")
	}
}

func TestReviewProjectorForbidden(t *testing.T) {
	projectID := uuid.New()
	review, task := newReviewFixture(projectID, model.ReviewStatusCompleted)
	p := NewReviewProjector(&stubReviewReader{review: review}, &stubTaskReader{task: task})

	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: ""}
	res, err := p.Project(context.Background(), pc, projectID, reviewRef(review.ID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusForbidden {
		t.Fatalf("want StatusForbidden, got %s", res.Status)
	}
	if len(res.Projection) != 0 {
		t.Fatalf("forbidden result must not leak projection JSON")
	}
}

func TestReviewProjectorNotFoundNilReview(t *testing.T) {
	p := NewReviewProjector(&stubReviewReader{review: nil}, &stubTaskReader{})
	res, err := p.Project(context.Background(), viewerPrincipal(), uuid.New(), reviewRef(uuid.New()), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusNotFound {
		t.Fatalf("want StatusNotFound, got %s", res.Status)
	}
}

func TestReviewProjectorNotFoundGormErr(t *testing.T) {
	p := NewReviewProjector(
		&stubReviewReader{review: nil, err: gorm.ErrRecordNotFound},
		&stubTaskReader{},
	)
	res, err := p.Project(context.Background(), viewerPrincipal(), uuid.New(), reviewRef(uuid.New()), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusNotFound {
		t.Fatalf("want StatusNotFound, got %s", res.Status)
	}
}

func TestReviewProjectorFindingsPreviewOff(t *testing.T) {
	projectID := uuid.New()
	review, task := newReviewFixture(projectID, model.ReviewStatusCompleted)
	review.Findings = []model.ReviewFinding{
		{Category: "security", Severity: model.ReviewRiskLevelCritical, Message: "sqli"},
		{Category: "style", Severity: model.ReviewRiskLevelLow, Message: "nit"},
	}
	p := NewReviewProjector(&stubReviewReader{review: review}, &stubTaskReader{task: task})

	opts, _ := json.Marshal(map[string]any{"show_findings_preview": false})
	res, err := p.Project(context.Background(), viewerPrincipal(), projectID, reviewRef(review.ID), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	text := flattenBlocks(t, res.Projection)
	if strings.Contains(text, "• [") {
		t.Fatalf("expected no per-finding bullets when preview off: %s", text)
	}
	if !strings.Contains(text, "Findings: 2 total") {
		t.Fatalf("expected findings count line: %s", text)
	}
}

func TestReviewProjectorFindingsPreviewOverflow(t *testing.T) {
	projectID := uuid.New()
	review, task := newReviewFixture(projectID, model.ReviewStatusCompleted)
	review.Findings = []model.ReviewFinding{
		{Category: "a", Severity: model.ReviewRiskLevelCritical, Message: "m1"},
		{Category: "b", Severity: model.ReviewRiskLevelHigh, Message: "m2"},
		{Category: "c", Severity: model.ReviewRiskLevelMedium, Message: "m3"},
		{Category: "d", Severity: model.ReviewRiskLevelLow, Message: "m4"},
		{Category: "e", Severity: model.ReviewRiskLevelLow, Message: "m5"},
	}
	p := NewReviewProjector(&stubReviewReader{review: review}, &stubTaskReader{task: task})

	res, err := p.Project(context.Background(), viewerPrincipal(), projectID, reviewRef(review.ID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	text := flattenBlocks(t, res.Projection)
	bullets := strings.Count(text, "• [")
	if bullets != 3 {
		t.Fatalf("expected 3 preview bullets, got %d: %s", bullets, text)
	}
	if !strings.Contains(text, "(2 more findings …)") {
		t.Fatalf("expected trailing overflow marker: %s", text)
	}
}

func TestReviewProjectorSubscribeScoped(t *testing.T) {
	p := NewReviewProjector(&stubReviewReader{}, &stubTaskReader{})
	reviewID := uuid.New()
	topics := p.Subscribe(reviewRef(reviewID))
	if len(topics) != 4 {
		t.Fatalf("want 4 topics, got %d", len(topics))
	}
	wantEvents := map[string]struct{}{
		"review.updated":       {},
		"review.completed":     {},
		"review.pending_human": {},
		"review.fix_requested": {},
	}
	for _, topic := range topics {
		if _, ok := wantEvents[topic.Event]; !ok {
			t.Errorf("unexpected event %q", topic.Event)
		}
		if topic.Scope["review_id"] != reviewID.String() {
			t.Errorf("topic %q missing review_id scope: %+v", topic.Event, topic.Scope)
		}
	}
}

func TestReviewProjectorSubscribeInvalidRef(t *testing.T) {
	p := NewReviewProjector(&stubReviewReader{}, &stubTaskReader{})
	topics := p.Subscribe(json.RawMessage(`{"kind":"review","id":"not-a-uuid"}`))
	if topics == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(topics) != 0 {
		t.Fatalf("expected empty topic slice, got %d", len(topics))
	}
}
