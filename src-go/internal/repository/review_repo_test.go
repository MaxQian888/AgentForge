package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubReviewRow struct {
	findings          []byte
	executionMetadata []byte
}

func (r stubReviewRow) Scan(dest ...any) error {
	now := time.Now().UTC()

	*(dest[0].(*uuid.UUID)) = uuid.New()
	*(dest[1].(*uuid.UUID)) = uuid.New()
	*(dest[2].(*string)) = "https://github.com/acme/project/pull/42"
	*(dest[3].(*int)) = 42
	*(dest[4].(*int)) = model.ReviewLayerDeep
	*(dest[5].(*string)) = model.ReviewStatusCompleted
	*(dest[6].(*string)) = model.ReviewRiskLevelHigh
	*(dest[7].(*[]byte)) = append([]byte(nil), r.findings...)
	*(dest[8].(*[]byte)) = append([]byte(nil), r.executionMetadata...)
	*(dest[9].(*string)) = "High-risk secret exposure"
	*(dest[10].(*string)) = model.ReviewRecommendationReject
	*(dest[11].(*float64)) = 1.15
	*(dest[12].(*time.Time)) = now
	*(dest[13].(*time.Time)) = now

	return nil
}

func TestNewReviewRepository(t *testing.T) {
	repo := NewReviewRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil ReviewRepository")
	}
}

func TestReviewRepositoryCreateNilDB(t *testing.T) {
	repo := NewReviewRepository(nil)
	err := repo.Create(context.Background(), &model.Review{ID: uuid.New(), TaskID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestReviewRepositoryUpdateResultNilDB(t *testing.T) {
	repo := NewReviewRepository(nil)
	err := repo.UpdateResult(context.Background(), &model.Review{ID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("UpdateResult() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestMarshalFindingsEmptyArray(t *testing.T) {
	data, err := marshalFindings(nil)
	if err != nil {
		t.Fatalf("marshalFindings(nil) error = %v", err)
	}
	if string(data) != "[]" {
		t.Fatalf("marshalFindings(nil) = %s, want []", string(data))
	}
}

func TestScanReviewUnmarshalsFindings(t *testing.T) {
	review, err := scanReview(stubReviewRow{
		findings: []byte(`[{"category":"security","severity":"high","message":"hardcoded secret"}]`),
	})
	if err != nil {
		t.Fatalf("scanReview() error = %v", err)
	}
	if len(review.Findings) != 1 {
		t.Fatalf("len(Findings) = %d, want 1", len(review.Findings))
	}
	if review.Findings[0].Category != "security" {
		t.Fatalf("Findings[0].Category = %q, want security", review.Findings[0].Category)
	}
}

func TestScanReviewUnmarshalsExecutionMetadata(t *testing.T) {
	review, err := scanReview(stubReviewRow{
		findings: []byte(`[]`),
		executionMetadata: []byte(`{
			"triggerEvent":"pull_request.updated",
			"changedFiles":["src/server/routes.go"],
			"dimensions":["logic","security"],
			"results":[
				{"id":"logic","kind":"builtin_dimension","status":"completed","summary":"logic ok"},
				{"id":"review.architecture","kind":"review_plugin","status":"failed","summary":"plugin failed","error":"timeout"}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("scanReview() error = %v", err)
	}
	if review.ExecutionMetadata == nil {
		t.Fatal("expected execution metadata to be populated")
	}
	if review.ExecutionMetadata.TriggerEvent != "pull_request.updated" {
		t.Fatalf("TriggerEvent = %q, want pull_request.updated", review.ExecutionMetadata.TriggerEvent)
	}
	if len(review.ExecutionMetadata.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(review.ExecutionMetadata.Results))
	}
	if review.ExecutionMetadata.Results[1].ID != "review.architecture" {
		t.Fatalf("Results[1].ID = %q, want review.architecture", review.ExecutionMetadata.Results[1].ID)
	}
}
