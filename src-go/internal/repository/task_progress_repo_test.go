package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestNewTaskProgressRepository(t *testing.T) {
	repo := NewTaskProgressRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil TaskProgressRepository")
	}
}

func TestTaskProgressRepositoryGetByTaskIDNilDB(t *testing.T) {
	repo := NewTaskProgressRepository(nil)
	_, err := repo.GetByTaskID(context.Background(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Fatalf("GetByTaskID() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestTaskProgressRepositoryUpsertNilDB(t *testing.T) {
	repo := NewTaskProgressRepository(nil)
	err := repo.Upsert(context.Background(), &model.TaskProgressSnapshot{TaskID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Upsert() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestTaskProgressRecordPreservesRiskFields(t *testing.T) {
	now := time.Now().UTC()
	riskSince := now.Add(-time.Hour)

	snapshot := &model.TaskProgressSnapshot{
		TaskID:             uuid.New(),
		LastActivityAt:     now,
		LastActivitySource: model.TaskProgressSourceAgentHeartbeat,
		LastTransitionAt:   now.Add(-2 * time.Minute),
		HealthStatus:       model.TaskProgressHealthWarning,
		RiskReason:         model.TaskProgressReasonNoRecentUpdate,
		RiskSinceAt:        &riskSince,
		LastAlertState:     "warning",
	}

	record := newTaskProgressSnapshotRecord(snapshot)
	result := record.toModel()

	if result.HealthStatus != model.TaskProgressHealthWarning {
		t.Fatalf("HealthStatus = %q, want %q", result.HealthStatus, model.TaskProgressHealthWarning)
	}
	if result.RiskReason != model.TaskProgressReasonNoRecentUpdate {
		t.Fatalf("RiskReason = %q, want %q", result.RiskReason, model.TaskProgressReasonNoRecentUpdate)
	}
	if result.RiskSinceAt == nil {
		t.Fatal("RiskSinceAt = nil, want non-nil")
	}
}
