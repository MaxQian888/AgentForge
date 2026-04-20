package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
)

type mockAggregationReviewLister struct {
	byTask  map[uuid.UUID][]*model.Review
	byID    map[uuid.UUID]*model.Review
	taskErr error
	idErr   error
}

func (m *mockAggregationReviewLister) GetByTask(_ context.Context, taskID uuid.UUID) ([]*model.Review, error) {
	if m.taskErr != nil {
		return nil, m.taskErr
	}
	reviews := m.byTask[taskID]
	out := make([]*model.Review, 0, len(reviews))
	for _, review := range reviews {
		cloned := *review
		cloned.Findings = append([]model.ReviewFinding(nil), review.Findings...)
		out = append(out, &cloned)
	}
	return out, nil
}

func (m *mockAggregationReviewLister) GetByID(_ context.Context, id uuid.UUID) (*model.Review, error) {
	if m.idErr != nil {
		return nil, m.idErr
	}
	review, ok := m.byID[id]
	if !ok {
		return nil, errors.New("review not found")
	}
	cloned := *review
	cloned.Findings = append([]model.ReviewFinding(nil), review.Findings...)
	return &cloned, nil
}

type mockAggregationRepo struct {
	byTask  map[uuid.UUID][]*model.ReviewAggregation
	created []*model.ReviewAggregation
	updated []*model.ReviewAggregation
}

func (m *mockAggregationRepo) Create(_ context.Context, agg *model.ReviewAggregation) error {
	cloned := cloneAggregation(agg)
	m.created = append(m.created, cloned)
	if m.byTask == nil {
		m.byTask = make(map[uuid.UUID][]*model.ReviewAggregation)
	}
	m.byTask[agg.TaskID] = []*model.ReviewAggregation{cloned}
	return nil
}

func (m *mockAggregationRepo) GetByID(_ context.Context, id uuid.UUID) (*model.ReviewAggregation, error) {
	for _, aggs := range m.byTask {
		for _, agg := range aggs {
			if agg.ID == id {
				return cloneAggregation(agg), nil
			}
		}
	}
	return nil, errors.New("aggregation not found")
}

func (m *mockAggregationRepo) GetByTask(_ context.Context, taskID uuid.UUID) ([]*model.ReviewAggregation, error) {
	aggs := m.byTask[taskID]
	out := make([]*model.ReviewAggregation, 0, len(aggs))
	for _, agg := range aggs {
		out = append(out, cloneAggregation(agg))
	}
	return out, nil
}

func (m *mockAggregationRepo) Update(_ context.Context, agg *model.ReviewAggregation) error {
	cloned := cloneAggregation(agg)
	m.updated = append(m.updated, cloned)
	if m.byTask == nil {
		m.byTask = make(map[uuid.UUID][]*model.ReviewAggregation)
	}
	m.byTask[agg.TaskID] = []*model.ReviewAggregation{cloned}
	return nil
}

type mockFalsePositiveRepo struct {
	byProject   map[uuid.UUID][]*model.FalsePositive
	created     []*model.FalsePositive
	incremented []uuid.UUID
	listErr     error
	createErr   error
}

func (m *mockFalsePositiveRepo) Create(_ context.Context, fp *model.FalsePositive) error {
	if m.createErr != nil {
		return m.createErr
	}
	cloned := *fp
	m.created = append(m.created, &cloned)
	return nil
}

func (m *mockFalsePositiveRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.FalsePositive, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	fps := m.byProject[projectID]
	out := make([]*model.FalsePositive, 0, len(fps))
	for _, fp := range fps {
		cloned := *fp
		out = append(out, &cloned)
	}
	return out, nil
}

func (m *mockFalsePositiveRepo) IncrementOccurrences(_ context.Context, id uuid.UUID) error {
	m.incremented = append(m.incremented, id)
	return nil
}

type mockAggregationTaskLookup struct {
	tasks map[uuid.UUID]*model.Task
	err   error
}

func (m *mockAggregationTaskLookup) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	if m.err != nil {
		return nil, m.err
	}
	task, ok := m.tasks[id]
	if !ok {
		return nil, errors.New("task not found")
	}
	cloned := *task
	return &cloned, nil
}

func TestReviewAggregationServiceAggregateCreatesDedupedAggregation(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	falsePositiveID := uuid.New()

	duplicate := model.ReviewFinding{Category: "logic", Severity: "high", File: "src-go/internal/service/team_service.go", Line: 42, Message: "duplicate finding"}
	suppressed := model.ReviewFinding{Category: "noise", Severity: "low", File: "docs/notes.md", Line: 8, Message: "ignore this transient warning"}
	unique := model.ReviewFinding{Category: "security", Severity: "critical", File: "src-go/internal/service/review_service.go", Line: 108, Message: "missing auth guard"}

	review1 := &model.Review{
		ID:             uuid.New(),
		TaskID:         taskID,
		PRURL:          "https://example.com/pr/91",
		Status:         model.ReviewStatusCompleted,
		RiskLevel:      model.ReviewRiskLevelHigh,
		Recommendation: model.ReviewRecommendationRequestChanges,
		CostUSD:        1.25,
		Findings:       []model.ReviewFinding{duplicate, suppressed},
	}
	review2 := &model.Review{
		ID:             uuid.New(),
		TaskID:         taskID,
		PRURL:          "https://example.com/pr/91",
		Status:         model.ReviewStatusCompleted,
		RiskLevel:      model.ReviewRiskLevelCritical,
		Recommendation: model.ReviewRecommendationReject,
		CostUSD:        2.5,
		Findings:       []model.ReviewFinding{duplicate, unique},
	}
	review3 := &model.Review{
		ID:             uuid.New(),
		TaskID:         taskID,
		Status:         model.ReviewStatusPending,
		RiskLevel:      model.ReviewRiskLevelLow,
		Recommendation: model.ReviewRecommendationApprove,
		CostUSD:        99,
		Findings:       []model.ReviewFinding{{Category: "pending", Message: "ignored"}},
	}

	reviews := &mockAggregationReviewLister{
		byTask: map[uuid.UUID][]*model.Review{taskID: {review1, review2, review3}},
	}
	aggRepo := &mockAggregationRepo{}
	fpRepo := &mockFalsePositiveRepo{
		byProject: map[uuid.UUID][]*model.FalsePositive{
			projectID: {
				{
					ID:          falsePositiveID,
					ProjectID:   projectID,
					Pattern:     "transient warning",
					Category:    "noise",
					FilePattern: "docs/",
				},
			},
		},
	}
	taskLookup := &mockAggregationTaskLookup{
		tasks: map[uuid.UUID]*model.Task{
			taskID: {ID: taskID, ProjectID: projectID},
		},
	}

	svc := service.NewReviewAggregationService(reviews, aggRepo, fpRepo, taskLookup)

	agg, err := svc.Aggregate(ctx, taskID)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	if agg == nil {
		t.Fatal("expected aggregation result")
	}
	if agg.OverallRisk != model.ReviewRiskLevelCritical {
		t.Fatalf("OverallRisk = %q, want %q", agg.OverallRisk, model.ReviewRiskLevelCritical)
	}
	if agg.Recommendation != model.ReviewRecommendationReject {
		t.Fatalf("Recommendation = %q, want %q", agg.Recommendation, model.ReviewRecommendationReject)
	}
	if agg.TotalCostUsd != 3.75 {
		t.Fatalf("TotalCostUsd = %.2f, want 3.75", agg.TotalCostUsd)
	}
	if len(agg.ReviewIDs) != 2 {
		t.Fatalf("ReviewIDs = %#v, want two completed reviews", agg.ReviewIDs)
	}
	if !strings.Contains(agg.Summary, "2 unique findings (1 suppressed)") {
		t.Fatalf("Summary = %q", agg.Summary)
	}
	if len(fpRepo.incremented) != 1 || fpRepo.incremented[0] != falsePositiveID {
		t.Fatalf("IncrementOccurrences() calls = %#v, want false positive %s", fpRepo.incremented, falsePositiveID)
	}
	if len(aggRepo.created) != 1 || len(aggRepo.updated) != 0 {
		t.Fatalf("aggregation persistence mismatch: created=%d updated=%d", len(aggRepo.created), len(aggRepo.updated))
	}

	var findings []model.ReviewFinding
	if err := json.Unmarshal([]byte(agg.Findings), &findings); err != nil {
		t.Fatalf("unmarshal findings: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("len(findings) = %d, want 2", len(findings))
	}
	if findings[0].Message != duplicate.Message || findings[1].Message != unique.Message {
		t.Fatalf("unexpected deduped findings: %#v", findings)
	}

	var metrics map[string]any
	if err := json.Unmarshal([]byte(agg.Metrics), &metrics); err != nil {
		t.Fatalf("unmarshal metrics: %v", err)
	}
	if metrics["totalReviews"] != float64(2) || metrics["totalFindings"] != float64(2) || metrics["suppressedCount"] != float64(1) {
		t.Fatalf("unexpected metrics payload: %#v", metrics)
	}
}

func TestReviewAggregationServiceAggregateUpdatesExistingRecord(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	existing := &model.ReviewAggregation{ID: uuid.New(), TaskID: taskID}

	review := &model.Review{
		ID:             uuid.New(),
		TaskID:         taskID,
		PRURL:          "https://example.com/pr/92",
		Status:         model.ReviewStatusCompleted,
		RiskLevel:      model.ReviewRiskLevelMedium,
		Recommendation: model.ReviewRecommendationApprove,
		CostUSD:        0.75,
		Findings:       []model.ReviewFinding{{Category: "style", Severity: "low", File: "src-go/main.go", Line: 7, Message: "nit"}},
	}

	svc := service.NewReviewAggregationService(
		&mockAggregationReviewLister{byTask: map[uuid.UUID][]*model.Review{taskID: {review}}},
		&mockAggregationRepo{byTask: map[uuid.UUID][]*model.ReviewAggregation{taskID: {existing}}},
		&mockFalsePositiveRepo{},
		&mockAggregationTaskLookup{tasks: map[uuid.UUID]*model.Task{taskID: {ID: taskID, ProjectID: projectID}}},
	)

	agg, err := svc.Aggregate(ctx, taskID)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}
	if agg.ID != existing.ID {
		t.Fatalf("updated aggregation ID = %s, want %s", agg.ID, existing.ID)
	}
}

func TestReviewAggregationServiceMarkFalsePositiveCreatesRepositoryRecord(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	reviewID := uuid.New()
	fpRepo := &mockFalsePositiveRepo{}

	svc := service.NewReviewAggregationService(
		&mockAggregationReviewLister{
			byID: map[uuid.UUID]*model.Review{
				reviewID: {
					ID:     reviewID,
					TaskID: taskID,
					Findings: []model.ReviewFinding{
						{Category: "security", File: "src-go/internal/service/review_service.go", Message: "missing auth guard"},
					},
				},
			},
		},
		&mockAggregationRepo{},
		fpRepo,
		&mockAggregationTaskLookup{tasks: map[uuid.UUID]*model.Task{taskID: {ID: taskID, ProjectID: projectID}}},
	)

	if err := svc.MarkFalsePositive(ctx, reviewID, 0, "accepted risk for internal admin flow"); err != nil {
		t.Fatalf("MarkFalsePositive() error = %v", err)
	}
	if len(fpRepo.created) != 1 {
		t.Fatalf("created false positives = %d, want 1", len(fpRepo.created))
	}
	created := fpRepo.created[0]
	if created.ProjectID != projectID || created.Pattern != "missing auth guard" || created.Category != "security" {
		t.Fatalf("unexpected false positive record: %+v", created)
	}
	if created.FilePattern != "src-go/internal/service/review_service.go" {
		t.Fatalf("FilePattern = %q", created.FilePattern)
	}
	if created.Reason != "accepted risk for internal admin flow" {
		t.Fatalf("Reason = %q", created.Reason)
	}
	if created.Occurrences != 1 || created.IsStrong {
		t.Fatalf("unexpected defaults on false positive: %+v", created)
	}
}

func cloneAggregation(agg *model.ReviewAggregation) *model.ReviewAggregation {
	if agg == nil {
		return nil
	}
	cloned := *agg
	if agg.ReviewIDs != nil {
		cloned.ReviewIDs = append([]uuid.UUID(nil), agg.ReviewIDs...)
	}
	return &cloned
}
