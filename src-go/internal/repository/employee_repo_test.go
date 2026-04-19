package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestNewEmployeeRepository_Constructs(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil EmployeeRepository")
	}
}

func TestEmployeeRepository_Create_NilDB(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	err := repo.Create(context.Background(), &model.Employee{ID: uuid.New()})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestEmployeeRepository_Get_NilDB(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	_, err := repo.Get(context.Background(), uuid.New())
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestEmployeeRepository_ListByProject_NilDB(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	_, err := repo.ListByProject(context.Background(), uuid.New(), repository.EmployeeFilter{})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestEmployeeRepository_Update_NilDB(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	err := repo.Update(context.Background(), &model.Employee{ID: uuid.New()})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestEmployeeRepository_SetState_NilDB(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	err := repo.SetState(context.Background(), uuid.New(), model.EmployeeStateActive)
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestEmployeeRepository_Delete_NilDB(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	err := repo.Delete(context.Background(), uuid.New())
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestEmployeeRepository_AddSkill_NilDB(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	err := repo.AddSkill(context.Background(), uuid.New(), model.EmployeeSkill{SkillPath: "test/skill"})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestEmployeeRepository_RemoveSkill_NilDB(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	err := repo.RemoveSkill(context.Background(), uuid.New(), "test/skill")
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestEmployeeRepository_ListSkills_NilDB(t *testing.T) {
	repo := repository.NewEmployeeRepository(nil)
	_, err := repo.ListSkills(context.Background(), uuid.New())
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}
