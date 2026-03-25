package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubAgentRunRow struct {
	roleID   string
	teamID   *uuid.UUID
	teamRole string
}

func (r stubAgentRunRow) Scan(dest ...any) error {
	now := time.Now().UTC()

	*(dest[0].(*uuid.UUID)) = uuid.New()
	*(dest[1].(*uuid.UUID)) = uuid.New()
	*(dest[2].(*uuid.UUID)) = uuid.New()
	*(dest[3].(*string)) = r.roleID
	*(dest[4].(*string)) = model.AgentRunStatusRunning
	*(dest[5].(*string)) = "codex"
	*(dest[6].(*string)) = "openai"
	*(dest[7].(*string)) = "gpt-5.4"
	*(dest[8].(*int64)) = 12
	*(dest[9].(*int64)) = 18
	*(dest[10].(*int64)) = 0
	*(dest[11].(*float64)) = 0.42
	*(dest[12].(*int)) = 3
	*(dest[13].(*string)) = ""
	*(dest[14].(*time.Time)) = now
	*(dest[15].(**time.Time)) = nil
	*(dest[16].(*time.Time)) = now
	*(dest[17].(*time.Time)) = now
	*(dest[18].(**uuid.UUID)) = r.teamID
	*(dest[19].(*string)) = r.teamRole

	return nil
}

func TestNewAgentRunRepository(t *testing.T) {
	repo := NewAgentRunRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil AgentRunRepository")
	}
}

func TestAgentRunRepositoryCreateNilDB(t *testing.T) {
	repo := NewAgentRunRepository(nil)
	err := repo.Create(context.Background(), &model.AgentRun{ID: uuid.New(), TaskID: uuid.New(), MemberID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestAgentRunRepositoryGetByIDNilDB(t *testing.T) {
	repo := NewAgentRunRepository(nil)
	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Fatalf("GetByID() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestScanAgentRunPreservesRoleAndTeamFields(t *testing.T) {
	teamID := uuid.New()

	run, err := scanAgentRun(stubAgentRunRow{
		roleID:   "frontend-developer",
		teamID:   &teamID,
		teamRole: model.TeamRoleCoder,
	})
	if err != nil {
		t.Fatalf("scanAgentRun() error = %v", err)
	}
	if run.RoleID != "frontend-developer" {
		t.Fatalf("RoleID = %q, want frontend-developer", run.RoleID)
	}
	if run.TeamID == nil || *run.TeamID != teamID {
		t.Fatalf("TeamID = %v, want %s", run.TeamID, teamID)
	}
	if run.TeamRole != model.TeamRoleCoder {
		t.Fatalf("TeamRole = %q, want %q", run.TeamRole, model.TeamRoleCoder)
	}
}
