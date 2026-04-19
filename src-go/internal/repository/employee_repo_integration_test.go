//go:build integration

package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/pkg/database"
)

// Note: TestMain is defined in user_repo_integration_test.go and runs
// migrations once for the entire package. Do NOT redefine it here.

func TestEmployeeRepository_Integration_CreateAndGet(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()

	// Insert a minimal projects row to satisfy the FK constraint.
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "test-project-"+projectID.String(), "slug-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM employees WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewEmployeeRepository(db)
	emp := &model.Employee{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        "test-employee",
		DisplayName: "Test Employee",
		RoleID:      "developer",
		State:       model.EmployeeStateActive,
	}

	if err := repo.Create(ctx, emp); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.Get(ctx, emp.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.ID != emp.ID {
		t.Errorf("ID: want %s, got %s", emp.ID, got.ID)
	}
	if got.Name != emp.Name {
		t.Errorf("Name: want %q, got %q", emp.Name, got.Name)
	}
	if got.RoleID != emp.RoleID {
		t.Errorf("RoleID: want %q, got %q", emp.RoleID, got.RoleID)
	}
}

func TestEmployeeRepository_Integration_UniqueProjectName(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "test-project-"+projectID.String(), "slug-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM employees WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewEmployeeRepository(db)
	first := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectID,
		Name:      "duplicate-name",
		RoleID:    "developer",
		State:     model.EmployeeStateActive,
	}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create() first: %v", err)
	}

	second := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectID,
		Name:      "duplicate-name",
		RoleID:    "developer",
		State:     model.EmployeeStateActive,
	}
	err = repo.Create(ctx, second)
	if err != repository.ErrEmployeeNameConflict {
		t.Errorf("expected ErrEmployeeNameConflict, got %v", err)
	}
}

func TestEmployeeRepository_Integration_ListByProject(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "test-project-"+projectID.String(), "slug-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM employees WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewEmployeeRepository(db)
	for i := 0; i < 3; i++ {
		emp := &model.Employee{
			ID:        uuid.New(),
			ProjectID: projectID,
			Name:      uuid.NewString(), // unique names
			RoleID:    "developer",
			State:     model.EmployeeStateActive,
		}
		if err := repo.Create(ctx, emp); err != nil {
			t.Fatalf("Create() employee %d: %v", i, err)
		}
	}

	list, err := repo.ListByProject(ctx, projectID, repository.EmployeeFilter{})
	if err != nil {
		t.Fatalf("ListByProject() error: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 employees, got %d", len(list))
	}
}

func TestEmployeeRepository_Integration_SetStateAndDelete(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "test-project-"+projectID.String(), "slug-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM employees WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewEmployeeRepository(db)
	emp := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectID,
		Name:      "state-test-employee",
		RoleID:    "developer",
		State:     model.EmployeeStateActive,
	}
	if err := repo.Create(ctx, emp); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.SetState(ctx, emp.ID, model.EmployeeStateArchived); err != nil {
		t.Fatalf("SetState() error: %v", err)
	}

	got, err := repo.Get(ctx, emp.ID)
	if err != nil {
		t.Fatalf("Get() after SetState error: %v", err)
	}
	if got.State != model.EmployeeStateArchived {
		t.Errorf("State: want %q, got %q", model.EmployeeStateArchived, got.State)
	}

	if err := repo.Delete(ctx, emp.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err = repo.Get(ctx, emp.ID)
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestEmployeeRepository_Integration_SkillsCRUD(t *testing.T) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("TEST_POSTGRES_URL not set — skipping integration test")
	}

	db, err := database.NewPostgres(url)
	if err != nil {
		t.Fatalf("NewPostgres() error: %v", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()

	ctx := context.Background()
	projectID := uuid.New()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)",
		projectID, "test-project-"+projectID.String(), "slug-"+projectID.String(),
	).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() {
		_ = db.WithContext(ctx).Exec("DELETE FROM employees WHERE project_id = ?", projectID).Error
		_ = db.WithContext(ctx).Exec("DELETE FROM projects WHERE id = ?", projectID).Error
	})

	repo := repository.NewEmployeeRepository(db)
	emp := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectID,
		Name:      "skills-test-employee",
		RoleID:    "developer",
		State:     model.EmployeeStateActive,
	}
	if err := repo.Create(ctx, emp); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	skill := model.EmployeeSkill{
		EmployeeID: emp.ID,
		SkillPath:  "tools/search",
		AutoLoad:   true,
	}
	if err := repo.AddSkill(ctx, emp.ID, skill); err != nil {
		t.Fatalf("AddSkill() error: %v", err)
	}

	// Upsert: second AddSkill with same path should not error.
	skill.AutoLoad = false
	if err := repo.AddSkill(ctx, emp.ID, skill); err != nil {
		t.Fatalf("AddSkill() upsert error: %v", err)
	}

	skills, err := repo.ListSkills(ctx, emp.ID)
	if err != nil {
		t.Fatalf("ListSkills() error: %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill after upsert, got %d", len(skills))
	}
	if skills[0].AutoLoad != false {
		t.Errorf("expected AutoLoad=false after upsert, got %v", skills[0].AutoLoad)
	}

	if err := repo.RemoveSkill(ctx, emp.ID, "tools/search"); err != nil {
		t.Fatalf("RemoveSkill() error: %v", err)
	}

	skills, err = repo.ListSkills(ctx, emp.ID)
	if err != nil {
		t.Fatalf("ListSkills() after remove error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills after remove, got %d", len(skills))
	}
}
