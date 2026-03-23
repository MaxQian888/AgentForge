package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestReviewRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewReviewRepository(mock)
	taskID := uuid.New()
	review := &model.Review{
		ID:       uuid.New(),
		TaskID:   taskID,
		PRURL:    "https://github.com/acme/project/pull/42",
		PRNumber: 42,
		Layer:    2,
		Status:   model.ReviewStatusInProgress,
		RiskLevel: model.ReviewRiskLevelMedium,
		Findings: []model.ReviewFinding{
			{
				Category: "logic",
				Severity: "medium",
				File:     "src/example.ts",
				Line:     12,
				Message:  "possible nil access",
			},
		},
		Summary:        "Found one logic issue",
		Recommendation: model.ReviewRecommendationRequestChanges,
		CostUSD:        0.42,
	}

	mock.ExpectExec("INSERT INTO reviews").
		WithArgs(
			review.ID,
			review.TaskID,
			review.PRURL,
			review.PRNumber,
			review.Layer,
			review.Status,
			review.RiskLevel,
			pgxmock.AnyArg(),
			review.Summary,
			review.Recommendation,
			review.CostUSD,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Create(context.Background(), review); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestReviewRepository_UpdateResult_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewReviewRepository(mock)
	review := &model.Review{
		ID:             uuid.New(),
		Status:         model.ReviewStatusCompleted,
		RiskLevel:      model.ReviewRiskLevelHigh,
		Findings:       []model.ReviewFinding{{Category: "security", Severity: "high", Message: "hardcoded secret"}},
		Summary:        "High-risk secret exposure",
		Recommendation: model.ReviewRecommendationReject,
		CostUSD:        1.15,
	}

	mock.ExpectExec("UPDATE reviews SET status =").
		WithArgs(
			review.Status,
			review.RiskLevel,
			pgxmock.AnyArg(),
			review.Summary,
			review.Recommendation,
			review.CostUSD,
			review.ID,
		).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.UpdateResult(context.Background(), review); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
