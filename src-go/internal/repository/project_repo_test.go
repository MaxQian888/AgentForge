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
		Status:        model.ProjectStatusActive,
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
		Status:        model.ProjectStatusActive,
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

func TestProjectRepository_ListHidesArchivedByDefault(t *testing.T) {
	db := openFoundationRepoTestDB(t, &projectRecord{}, &taskRecord{}, &memberRecord{})
	repo := NewProjectRepository(db)

	activeID := uuid.New()
	archivedID := uuid.New()
	now := time.Now().UTC()
	archivedAt := now.Add(-time.Hour)

	if err := db.Create(&projectRecord{
		ID:        activeID,
		Name:      "Active",
		Slug:      "active",
		Settings:  newJSONText("{}", "{}"),
		Status:    model.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create active project: %v", err)
	}
	if err := db.Create(&projectRecord{
		ID:         archivedID,
		Name:       "Archived",
		Slug:       "archived",
		Settings:   newJSONText("{}", "{}"),
		Status:     model.ProjectStatusArchived,
		ArchivedAt: &archivedAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("create archived project: %v", err)
	}

	defaulted, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List default: %v", err)
	}
	if len(defaulted) != 1 || defaulted[0].ID != activeID {
		t.Fatalf("default List should return only the active project, got %+v", defaulted)
	}

	withArchived, err := repo.ListWithFilter(context.Background(), ProjectListFilter{
		Statuses: []string{
			model.ProjectStatusActive,
			model.ProjectStatusPaused,
			model.ProjectStatusArchived,
		},
	})
	if err != nil {
		t.Fatalf("ListWithFilter all-statuses: %v", err)
	}
	if len(withArchived) != 2 {
		t.Fatalf("ListWithFilter all-statuses should return both projects, got %d", len(withArchived))
	}
}

func TestProjectRepository_SetArchivedAndUnarchivedRoundTrip(t *testing.T) {
	db := openFoundationRepoTestDB(t, &projectRecord{}, &taskRecord{}, &memberRecord{})
	repo := NewProjectRepository(db)

	projectID := uuid.New()
	ownerID := uuid.New()
	now := time.Now().UTC()

	if err := db.Create(&projectRecord{
		ID:        projectID,
		Name:      "p",
		Slug:      "p",
		Settings:  newJSONText("{}", "{}"),
		Status:    model.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}

	archivedAt := now
	if err := repo.SetArchived(context.Background(), projectID, ownerID, archivedAt); err != nil {
		t.Fatalf("SetArchived: %v", err)
	}
	got, err := repo.GetByID(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetByID after archive: %v", err)
	}
	if got.Status != model.ProjectStatusArchived {
		t.Errorf("after SetArchived, Status = %q, want archived", got.Status)
	}
	if got.ArchivedAt == nil || got.ArchivedByUserID == nil {
		t.Errorf("expected archived_at and archived_by_user_id set")
	}

	if err := repo.SetUnarchived(context.Background(), projectID); err != nil {
		t.Fatalf("SetUnarchived: %v", err)
	}
	got, err = repo.GetByID(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetByID after unarchive: %v", err)
	}
	if got.Status != model.ProjectStatusActive {
		t.Errorf("after SetUnarchived, Status = %q, want active", got.Status)
	}
	if got.ArchivedAt != nil || got.ArchivedByUserID != nil {
		t.Errorf("expected archived_at and archived_by_user_id cleared")
	}
}
