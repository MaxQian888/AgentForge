package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubAgentTeamRow struct {
	plannerRunID  *uuid.UUID
	reviewerRunID *uuid.UUID
}

func (r stubAgentTeamRow) Scan(dest ...any) error {
	now := time.Now().UTC()

	*(dest[0].(*uuid.UUID)) = uuid.New()
	*(dest[1].(*uuid.UUID)) = uuid.New()
	*(dest[2].(*uuid.UUID)) = uuid.New()
	*(dest[3].(*string)) = "Task Delivery Team"
	*(dest[4].(*string)) = model.TeamStatusExecuting
	*(dest[5].(*string)) = "plan-code-review"
	*(dest[6].(**uuid.UUID)) = r.plannerRunID
	*(dest[7].(**uuid.UUID)) = r.reviewerRunID
	*(dest[8].(*float64)) = 12.5
	*(dest[9].(*float64)) = 4.2
	*(dest[10].(*string)) = `{"runtime":"codex","provider":"openai","model":"gpt-5.4"}`
	*(dest[11].(*string)) = ""
	*(dest[12].(*time.Time)) = now
	*(dest[13].(*time.Time)) = now

	return nil
}

func TestNewAgentTeamRepository(t *testing.T) {
	repo := NewAgentTeamRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil AgentTeamRepository")
	}
}

func TestAgentTeamRepositoryCreateNilDB(t *testing.T) {
	repo := NewAgentTeamRepository(nil)
	err := repo.Create(context.Background(), &model.AgentTeam{ID: uuid.New(), ProjectID: uuid.New(), TaskID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestAgentTeamRepositoryGetByIDNilDB(t *testing.T) {
	repo := NewAgentTeamRepository(nil)
	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Fatalf("GetByID() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestScanAgentTeamPreservesPlannerAndReviewerRuns(t *testing.T) {
	plannerRunID := uuid.New()
	reviewerRunID := uuid.New()

	team, err := scanAgentTeam(stubAgentTeamRow{
		plannerRunID:  &plannerRunID,
		reviewerRunID: &reviewerRunID,
	})
	if err != nil {
		t.Fatalf("scanAgentTeam() error = %v", err)
	}
	if team.PlannerRunID == nil || *team.PlannerRunID != plannerRunID {
		t.Fatalf("PlannerRunID = %v, want %s", team.PlannerRunID, plannerRunID)
	}
	if team.ReviewerRunID == nil || *team.ReviewerRunID != reviewerRunID {
		t.Fatalf("ReviewerRunID = %v, want %s", team.ReviewerRunID, reviewerRunID)
	}
}
