package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestProjectRepository_ListAndGetByIDIncludeManagementSummary(t *testing.T) {
	db := openFoundationRepoTestDB(t, &projectRecord{}, &taskRecord{}, &memberRecord{})
	repo := NewProjectRepository(db)

	projectID := uuid.New()
	otherProjectID := uuid.New()
	now := time.Now().UTC()

	if err := db.Create(&projectRecord{
		ID:            projectID,
		Name:          "AgentForge",
		Slug:          "agentforge",
		Description:   "Main project",
		DefaultBranch: "main",
		Settings:      newJSONText("{}", "{}"),
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("create project record: %v", err)
	}
	if err := db.Create(&projectRecord{
		ID:            otherProjectID,
		Name:          "Docs",
		Slug:          "docs",
		Description:   "Docs project",
		DefaultBranch: "main",
		Settings:      newJSONText("{}", "{}"),
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("create other project record: %v", err)
	}

	taskRecords := []taskRecord{
		{ID: uuid.New(), ProjectID: projectID, Title: "Task 1", Status: model.TaskStatusInbox, Priority: "medium", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), ProjectID: projectID, Title: "Task 2", Status: model.TaskStatusInProgress, Priority: "high", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), ProjectID: otherProjectID, Title: "Task 3", Status: model.TaskStatusDone, Priority: "low", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&taskRecords).Error; err != nil {
		t.Fatalf("create task records: %v", err)
	}

	memberRecords := []memberRecord{
		{ID: uuid.New(), ProjectID: projectID, Name: "Alice", Type: model.MemberTypeHuman, Role: "pm", Status: model.MemberStatusActive, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), ProjectID: projectID, Name: "Builder Bot", Type: model.MemberTypeAgent, Role: "coder", Status: model.MemberStatusActive, IsActive: true, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), ProjectID: projectID, Name: "Review Bot", Type: model.MemberTypeAgent, Role: "reviewer", Status: model.MemberStatusSuspended, IsActive: false, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), ProjectID: otherProjectID, Name: "Docs Bot", Type: model.MemberTypeAgent, Role: "writer", Status: model.MemberStatusActive, IsActive: true, CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&memberRecords).Error; err != nil {
		t.Fatalf("create member records: %v", err)
	}

	projects, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var listed *model.Project
	for _, project := range projects {
		if project.ID == projectID {
			listed = project
			break
		}
	}
	if listed == nil {
		t.Fatalf("expected project %s in List()", projectID)
	}
	if listed.Status != "active" {
		t.Fatalf("listed.Status = %q, want active", listed.Status)
	}
	if listed.TaskCount != 2 {
		t.Fatalf("listed.TaskCount = %d, want 2", listed.TaskCount)
	}
	if listed.AgentCount != 2 {
		t.Fatalf("listed.AgentCount = %d, want 2", listed.AgentCount)
	}

	got, err := repo.GetByID(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("got.Status = %q, want active", got.Status)
	}
	if got.TaskCount != 2 {
		t.Fatalf("got.TaskCount = %d, want 2", got.TaskCount)
	}
	if got.AgentCount != 2 {
		t.Fatalf("got.AgentCount = %d, want 2", got.AgentCount)
	}
}
