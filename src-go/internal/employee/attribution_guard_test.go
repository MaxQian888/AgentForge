package employee_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentforge/server/internal/employee"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

func TestAttributionGuard_ValidateForProject(t *testing.T) {
	projectA := uuid.New()
	projectB := uuid.New()

	active := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectA,
		State:     model.EmployeeStateActive,
	}
	paused := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectA,
		State:     model.EmployeeStatePaused,
	}
	archived := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectA,
		State:     model.EmployeeStateArchived,
	}
	crossProject := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectB,
		State:     model.EmployeeStateActive,
	}

	repo := newMockEmployeeRepo()
	for _, e := range []*model.Employee{active, paused, archived, crossProject} {
		ec := *e
		repo.employees[e.ID] = &ec
	}

	g := employee.NewAttributionGuard(repo)
	ctx := context.Background()

	tests := []struct {
		name      string
		empID     uuid.UUID
		projectID uuid.UUID
		wantErr   error
	}{
		{"active in same project", active.ID, projectA, nil},
		{"paused in same project is accepted", paused.ID, projectA, nil},
		{"archived in same project rejected", archived.ID, projectA, employee.ErrEmployeeArchived},
		{"cross-project rejected", crossProject.ID, projectA, employee.ErrEmployeeCrossProject},
		{"unknown id rejected", uuid.New(), projectA, employee.ErrEmployeeNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := g.ValidateForProject(ctx, tc.empID, tc.projectID)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestAttributionGuard_ValidateNotArchived(t *testing.T) {
	projectID := uuid.New()

	active := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectID,
		State:     model.EmployeeStateActive,
	}
	paused := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectID,
		State:     model.EmployeeStatePaused,
	}
	archived := &model.Employee{
		ID:        uuid.New(),
		ProjectID: projectID,
		State:     model.EmployeeStateArchived,
	}

	repo := newMockEmployeeRepo()
	for _, e := range []*model.Employee{active, paused, archived} {
		ec := *e
		repo.employees[e.ID] = &ec
	}

	g := employee.NewAttributionGuard(repo)
	ctx := context.Background()

	tests := []struct {
		name    string
		empID   uuid.UUID
		wantErr error
	}{
		{"active accepted", active.ID, nil},
		{"paused accepted", paused.ID, nil},
		{"archived rejected", archived.ID, employee.ErrEmployeeArchived},
		{"unknown id rejected", uuid.New(), employee.ErrEmployeeNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := g.ValidateNotArchived(ctx, tc.empID)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestAttributionGuard_ValidateForProject_PassesThroughRepoError(t *testing.T) {
	repo := newMockEmployeeRepo()
	repo.getErr = errors.New("boom")

	g := employee.NewAttributionGuard(repo)
	err := g.ValidateForProject(context.Background(), uuid.New(), uuid.New())
	if err == nil || errors.Is(err, employee.ErrEmployeeNotFound) {
		t.Fatalf("unexpected error %v; want non-ErrEmployeeNotFound passthrough", err)
	}
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("ErrNotFound should be translated, got %v", err)
	}
}
