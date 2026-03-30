package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

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

func TestAgentTeamRecordPreservesPlannerAndReviewerRuns(t *testing.T) {
	plannerRunID := uuid.New()
	reviewerRunID := uuid.New()

	team := &model.AgentTeam{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		TaskID:        uuid.New(),
		Name:          "Task Delivery Team",
		Status:        model.TeamStatusExecuting,
		Strategy:      "plan-code-review",
		PlannerRunID:  &plannerRunID,
		ReviewerRunID: &reviewerRunID,
		Config:        `{"runtime":"codex","provider":"openai","model":"gpt-5.4"}`,
	}

	record := newAgentTeamRecord(team)
	result := record.toModel()

	if result.PlannerRunID == nil || *result.PlannerRunID != plannerRunID {
		t.Fatalf("PlannerRunID = %v, want %s", result.PlannerRunID, plannerRunID)
	}
	if result.ReviewerRunID == nil || *result.ReviewerRunID != reviewerRunID {
		t.Fatalf("ReviewerRunID = %v, want %s", result.ReviewerRunID, reviewerRunID)
	}
}

func TestAgentTeamRepositoryGetTeamSummaryPopulatesTaskAndRunFields(t *testing.T) {
	ctx := context.Background()
	db := openFoundationRepoTestDB(t, &taskRecord{}, &agentTeamRecord{}, &agentRunRecord{})
	repo := NewAgentTeamRepository(db)

	now := time.Date(2026, 3, 30, 9, 0, 0, 0, time.UTC)
	projectID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()
	plannerRunID := uuid.New()
	reviewerRunID := uuid.New()

	if err := db.Create(&taskRecord{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Review queue",
		Status:    "in_progress",
		Priority:  "high",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	if err := db.Create(&agentTeamRecord{
		ID:             teamID,
		ProjectID:      projectID,
		TaskID:         taskID,
		Name:           "Review queue team",
		Status:         model.TeamStatusExecuting,
		Strategy:       "plan-code-review",
		PlannerRunID:   &plannerRunID,
		ReviewerRunID:  &reviewerRunID,
		TotalBudgetUsd: 50,
		TotalSpentUsd:  18.5,
		Config:         newJSONText(`{"runtime":"codex","provider":"openai","model":"gpt-5.4"}`, "{}"),
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatalf("seed team: %v", err)
	}

	runRecords := []agentRunRecord{
		{
			ID:        plannerRunID,
			TaskID:    taskID,
			MemberID:  uuid.New(),
			Status:    model.AgentRunStatusCompleted,
			Runtime:   "codex",
			Provider:  "openai",
			Model:     "gpt-5.4",
			StartedAt: now,
			CreatedAt: now,
			UpdatedAt: now,
			TeamID:    &teamID,
			TeamRole:  model.TeamRolePlanner,
		},
		{
			ID:        uuid.New(),
			TaskID:    taskID,
			MemberID:  uuid.New(),
			Status:    model.AgentRunStatusCompleted,
			Runtime:   "codex",
			Provider:  "openai",
			Model:     "gpt-5.4",
			CostUsd:   7.25,
			StartedAt: now,
			CreatedAt: now,
			UpdatedAt: now,
			TeamID:    &teamID,
			TeamRole:  model.TeamRoleCoder,
		},
		{
			ID:        uuid.New(),
			TaskID:    taskID,
			MemberID:  uuid.New(),
			Status:    model.AgentRunStatusRunning,
			Runtime:   "codex",
			Provider:  "openai",
			Model:     "gpt-5.4",
			CostUsd:   3.10,
			StartedAt: now,
			CreatedAt: now,
			UpdatedAt: now,
			TeamID:    &teamID,
			TeamRole:  model.TeamRoleCoder,
		},
		{
			ID:        reviewerRunID,
			TaskID:    taskID,
			MemberID:  uuid.New(),
			Status:    model.AgentRunStatusRunning,
			Runtime:   "codex",
			Provider:  "openai",
			Model:     "gpt-5.4",
			StartedAt: now,
			CreatedAt: now,
			UpdatedAt: now,
			TeamID:    &teamID,
			TeamRole:  model.TeamRoleReviewer,
		},
	}
	for _, runRecord := range runRecords {
		if err := db.Create(&runRecord).Error; err != nil {
			t.Fatalf("seed run %s: %v", runRecord.ID, err)
		}
	}

	summary, err := repo.GetTeamSummary(ctx, teamID)
	if err != nil {
		t.Fatalf("GetTeamSummary() error = %v", err)
	}

	if summary.TaskTitle != "Review queue" {
		t.Fatalf("TaskTitle = %q, want Review queue", summary.TaskTitle)
	}
	if summary.PlannerStatus != model.AgentRunStatusCompleted {
		t.Fatalf("PlannerStatus = %q, want %q", summary.PlannerStatus, model.AgentRunStatusCompleted)
	}
	if summary.ReviewerStatus != model.AgentRunStatusRunning {
		t.Fatalf("ReviewerStatus = %q, want %q", summary.ReviewerStatus, model.AgentRunStatusRunning)
	}
	if len(summary.CoderRuns) != 2 {
		t.Fatalf("len(CoderRuns) = %d, want 2", len(summary.CoderRuns))
	}
	if summary.CoderTotal != 2 {
		t.Fatalf("CoderTotal = %d, want 2", summary.CoderTotal)
	}
	if summary.CoderCompleted != 1 {
		t.Fatalf("CoderCompleted = %d, want 1", summary.CoderCompleted)
	}
}

func TestAgentTeamRepositoryListTeamSummariesFiltersAndPopulatesTaskTitles(t *testing.T) {
	ctx := context.Background()
	db := openFoundationRepoTestDB(t, &taskRecord{}, &agentTeamRecord{}, &agentRunRecord{})
	repo := NewAgentTeamRepository(db)

	now := time.Date(2026, 3, 30, 9, 30, 0, 0, time.UTC)
	projectID := uuid.New()

	executingTaskID := uuid.New()
	completedTaskID := uuid.New()
	otherProjectTaskID := uuid.New()
	if err := db.Create([]taskRecord{
		{
			ID:        executingTaskID,
			ProjectID: projectID,
			Title:     "Executing queue",
			Status:    "in_progress",
			Priority:  "high",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        completedTaskID,
			ProjectID: projectID,
			Title:     "Completed queue",
			Status:    "done",
			Priority:  "medium",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        otherProjectTaskID,
			ProjectID: uuid.New(),
			Title:     "Other project queue",
			Status:    "triaged",
			Priority:  "low",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}

	executingTeamID := uuid.New()
	completedTeamID := uuid.New()
	otherProjectTeamID := uuid.New()
	if err := db.Create([]agentTeamRecord{
		{
			ID:             executingTeamID,
			ProjectID:      projectID,
			TaskID:         executingTaskID,
			Name:           "Executing team",
			Status:         model.TeamStatusExecuting,
			Strategy:       "plan-code-review",
			TotalBudgetUsd: 40,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             completedTeamID,
			ProjectID:      projectID,
			TaskID:         completedTaskID,
			Name:           "Completed team",
			Status:         model.TeamStatusCompleted,
			Strategy:       "plan-code-review",
			TotalBudgetUsd: 40,
			CreatedAt:      now.Add(time.Minute),
			UpdatedAt:      now.Add(time.Minute),
		},
		{
			ID:             otherProjectTeamID,
			ProjectID:      uuid.New(),
			TaskID:         otherProjectTaskID,
			Name:           "Other project team",
			Status:         model.TeamStatusExecuting,
			Strategy:       "plan-code-review",
			TotalBudgetUsd: 40,
			CreatedAt:      now.Add(2 * time.Minute),
			UpdatedAt:      now.Add(2 * time.Minute),
		},
	}).Error; err != nil {
		t.Fatalf("seed teams: %v", err)
	}

	summaries, err := repo.ListTeamSummaries(ctx, projectID, model.TeamStatusExecuting)
	if err != nil {
		t.Fatalf("ListTeamSummaries() error = %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(summaries))
	}
	if summaries[0].ID != executingTeamID.String() {
		t.Fatalf("summaries[0].ID = %q, want %q", summaries[0].ID, executingTeamID.String())
	}
	if summaries[0].TaskTitle != "Executing queue" {
		t.Fatalf("summaries[0].TaskTitle = %q, want Executing queue", summaries[0].TaskTitle)
	}
}
