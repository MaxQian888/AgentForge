package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

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

func TestReviewRepositorySetExecutionIDNilDB(t *testing.T) {
	repo := NewReviewRepository(nil)
	err := repo.SetExecutionID(context.Background(), uuid.New(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Errorf("SetExecutionID() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestNewReviewRecordEmptyFindings(t *testing.T) {
	record, err := newReviewRecord(&model.Review{
		ID:     uuid.New(),
		TaskID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("newReviewRecord() error = %v", err)
	}
	if string(record.Findings.Bytes("[]")) != "[]" {
		t.Fatalf("Findings = %s, want []", string(record.Findings.Bytes("[]")))
	}
}

func TestReviewRecordRoundtripFindings(t *testing.T) {
	review := &model.Review{
		ID:     uuid.New(),
		TaskID: uuid.New(),
		Findings: []model.ReviewFinding{
			{Category: "security", Severity: "high", Message: "hardcoded secret"},
		},
	}

	record, err := newReviewRecord(review)
	if err != nil {
		t.Fatalf("newReviewRecord() error = %v", err)
	}

	result, err := record.toModel()
	if err != nil {
		t.Fatalf("toModel() error = %v", err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("len(Findings) = %d, want 1", len(result.Findings))
	}
	if result.Findings[0].Category != "security" {
		t.Fatalf("Findings[0].Category = %q, want security", result.Findings[0].Category)
	}
}

func TestReviewRecordRoundtripExecutionMetadata(t *testing.T) {
	review := &model.Review{
		ID:     uuid.New(),
		TaskID: uuid.New(),
		ExecutionMetadata: &model.ReviewExecutionMetadata{
			TriggerEvent: "pull_request.updated",
			ChangedFiles: []string{"src/server/routes.go"},
			Dimensions:   []string{"logic", "security"},
			Results: []model.ReviewExecutionResult{
				{ID: "logic", Kind: model.ReviewExecutionKindBuiltinDimension, Status: model.ReviewExecutionStatusCompleted, Summary: "logic ok"},
				{ID: "review.architecture", Kind: model.ReviewExecutionKindPlugin, Status: model.ReviewExecutionStatusFailed, Summary: "plugin failed", Error: "timeout"},
			},
		},
	}

	record, err := newReviewRecord(review)
	if err != nil {
		t.Fatalf("newReviewRecord() error = %v", err)
	}

	// Verify the raw JSON was created
	rawBytes := record.ExecutionMetadata.Bytes("{}")
	if !json.Valid(rawBytes) {
		t.Fatalf("ExecutionMetadata JSON is invalid: %s", string(rawBytes))
	}

	result, err := record.toModel()
	if err != nil {
		t.Fatalf("toModel() error = %v", err)
	}
	if result.ExecutionMetadata == nil {
		t.Fatal("expected execution metadata to be populated")
	}
	if result.ExecutionMetadata.TriggerEvent != "pull_request.updated" {
		t.Fatalf("TriggerEvent = %q, want pull_request.updated", result.ExecutionMetadata.TriggerEvent)
	}
	if len(result.ExecutionMetadata.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(result.ExecutionMetadata.Results))
	}
	if result.ExecutionMetadata.Results[1].ID != "review.architecture" {
		t.Fatalf("Results[1].ID = %q, want review.architecture", result.ExecutionMetadata.Results[1].ID)
	}
}
