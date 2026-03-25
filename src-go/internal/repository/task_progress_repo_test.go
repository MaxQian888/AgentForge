package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubTaskProgressRow struct{}

func (stubTaskProgressRow) Scan(dest ...any) error {
	now := time.Now().UTC()
	riskSince := now.Add(-time.Hour)

	*(dest[0].(*uuid.UUID)) = uuid.New()
	*(dest[1].(*time.Time)) = now
	*(dest[2].(*string)) = model.TaskProgressSourceAgentHeartbeat
	*(dest[3].(*time.Time)) = now.Add(-2 * time.Minute)
	*(dest[4].(*string)) = model.TaskProgressHealthWarning
	*(dest[5].(*string)) = model.TaskProgressReasonNoRecentUpdate
	*(dest[6].(**time.Time)) = &riskSince
	*(dest[7].(*string)) = "warning"
	*(dest[8].(**time.Time)) = nil
	*(dest[9].(**time.Time)) = nil
	*(dest[10].(*time.Time)) = now
	*(dest[11].(*time.Time)) = now

	return nil
}

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

func TestScanTaskProgressPreservesRiskFields(t *testing.T) {
	snapshot, err := scanTaskProgress(stubTaskProgressRow{})
	if err != nil {
		t.Fatalf("scanTaskProgress() error = %v", err)
	}
	if snapshot.HealthStatus != model.TaskProgressHealthWarning {
		t.Fatalf("HealthStatus = %q, want %q", snapshot.HealthStatus, model.TaskProgressHealthWarning)
	}
	if snapshot.RiskReason != model.TaskProgressReasonNoRecentUpdate {
		t.Fatalf("RiskReason = %q, want %q", snapshot.RiskReason, model.TaskProgressReasonNoRecentUpdate)
	}
	if snapshot.RiskSinceAt == nil {
		t.Fatal("RiskSinceAt = nil, want non-nil")
	}
}
