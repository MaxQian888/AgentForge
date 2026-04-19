package employee_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/employee"
	"github.com/react-go-quick-starter/server/internal/repository"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newRegistry(repo *mockEmployeeRepo, roles *mockRoleRegistry) *employee.Registry {
	svc := employee.NewService(repo, roles, nil)
	return employee.NewRegistry(svc)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

const validManifest = `
apiVersion: agentforge/v1
kind: Employee
metadata:
  id: default-code-reviewer
  name: 默认代码评审员
  displayName: Default Code Reviewer
role_id: code-reviewer
runtime_prefs:
  runtime: claude_code
  provider: anthropic
  model: claude-opus-4-7
  budgetUsd: 7.5
config: {}
extra_skills:
  - path: skills/typescript
    auto_load: true
`

// ---------------------------------------------------------------------------
// Test 1 — upserts across multiple projects; idempotent on second run
// ---------------------------------------------------------------------------

func TestRegistry_SeedFromDir_UpsertsAcrossProjects(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "reviewer.yaml", validManifest)

	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("code-reviewer")
	reg := newRegistry(repo, roles)

	projectA := uuid.New()
	projectB := uuid.New()
	projects := []uuid.UUID{projectA, projectB}

	ctx := context.Background()

	// First seed run
	report, err := reg.SeedFromDir(ctx, dir, projects)
	if err != nil {
		t.Fatalf("SeedFromDir: unexpected error: %v", err)
	}
	if report.Upserted != 2 {
		t.Errorf("expected Upserted=2, got %d (errors: %v)", report.Upserted, report.Errors)
	}
	if report.Skipped != 0 {
		t.Errorf("expected Skipped=0, got %d", report.Skipped)
	}

	// Verify one employee per project
	empA, err := svcFromRegistry(repo, roles).ListByProject(ctx, projectA, repository.EmployeeFilter{})
	if err != nil || len(empA) != 1 {
		t.Errorf("expected 1 employee in projectA, got %d (err: %v)", len(empA), err)
	}
	empB, err := svcFromRegistry(repo, roles).ListByProject(ctx, projectB, repository.EmployeeFilter{})
	if err != nil || len(empB) != 1 {
		t.Errorf("expected 1 employee in projectB, got %d (err: %v)", len(empB), err)
	}

	// Second seed run — must be fully idempotent
	report2, err := reg.SeedFromDir(ctx, dir, projects)
	if err != nil {
		t.Fatalf("second SeedFromDir: unexpected error: %v", err)
	}
	if report2.Upserted != 0 {
		t.Errorf("expected Upserted=0 on second run, got %d", report2.Upserted)
	}
	if report2.Skipped != 2 {
		t.Errorf("expected Skipped=2 on second run, got %d", report2.Skipped)
	}
}

// svcFromRegistry returns a Service using the same repo/roles; used for
// verification queries without going through the Registry's private svc field.
func svcFromRegistry(repo *mockEmployeeRepo, roles *mockRoleRegistry) *employee.Service {
	return employee.NewService(repo, roles, nil)
}

// ---------------------------------------------------------------------------
// Test 2 — rejects manifests with wrong Kind
// ---------------------------------------------------------------------------

func TestRegistry_SeedFromDir_RejectsNonEmployeeKind(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "wrong.yaml", `
apiVersion: agentforge/v1
kind: NotEmployee
metadata:
  id: some-id
role_id: some-role
`)

	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("some-role")
	reg := newRegistry(repo, roles)

	report, err := reg.SeedFromDir(context.Background(), dir, []uuid.UUID{uuid.New()})
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if len(report.Errors) == 0 {
		t.Error("expected at least one error in report.Errors for wrong Kind")
	}
	if report.Upserted != 0 {
		t.Errorf("expected Upserted=0, got %d", report.Upserted)
	}
}

// ---------------------------------------------------------------------------
// Test 3 — non-YAML files are silently ignored
// ---------------------------------------------------------------------------

func TestRegistry_SeedFromDir_SkipsNonYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "readme.txt", "this is not a manifest")

	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry()
	reg := newRegistry(repo, roles)

	report, err := reg.SeedFromDir(context.Background(), dir, []uuid.UUID{uuid.New()})
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Errorf("expected no errors for txt file, got: %v", report.Errors)
	}
	if report.Upserted != 0 || report.Skipped != 0 {
		t.Errorf("expected no upserts or skips, got upserted=%d skipped=%d", report.Upserted, report.Skipped)
	}
}

// ---------------------------------------------------------------------------
// Test 4 — non-existent directory returns a top-level error
// ---------------------------------------------------------------------------

func TestRegistry_SeedFromDir_DirNotFound(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry()
	reg := newRegistry(repo, roles)

	_, err := reg.SeedFromDir(context.Background(), "/nonexistent/path/employees", []uuid.UUID{uuid.New()})
	if err == nil {
		t.Fatal("expected a top-level error for non-existent directory, got nil")
	}
}

// ---------------------------------------------------------------------------
// Test 5 — missing required fields go into report.Errors, not top-level error
// ---------------------------------------------------------------------------

func TestRegistry_SeedFromDir_MissingRequiredFields(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
	}{
		{
			name: "blank metadata.id",
			manifest: `
apiVersion: agentforge/v1
kind: Employee
metadata:
  id: ""
role_id: code-reviewer
`,
		},
		{
			name: "blank role_id",
			manifest: `
apiVersion: agentforge/v1
kind: Employee
metadata:
  id: some-employee
role_id: ""
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "manifest.yaml", tc.manifest)

			repo := newMockEmployeeRepo()
			roles := newMockRoleRegistry("code-reviewer")
			reg := newRegistry(repo, roles)

			report, err := reg.SeedFromDir(context.Background(), dir, []uuid.UUID{uuid.New()})
			if err != nil {
				t.Fatalf("expected nil top-level error, got: %v", err)
			}
			if len(report.Errors) == 0 {
				t.Errorf("expected error in report.Errors for %s", tc.name)
			}
			if report.Upserted != 0 {
				t.Errorf("expected Upserted=0, got %d", report.Upserted)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 6 — extra_skills are attached to the seeded employee
// ---------------------------------------------------------------------------

func TestRegistry_SeedFromDir_BuildsSkillsFromExtraSkills(t *testing.T) {
	manifest := `
apiVersion: agentforge/v1
kind: Employee
metadata:
  id: multi-skill-bot
  displayName: Multi Skill Bot
role_id: code-reviewer
extra_skills:
  - path: skills/typescript
    auto_load: true
  - path: skills/go
    auto_load: false
`
	dir := t.TempDir()
	writeFile(t, dir, "multi.yaml", manifest)

	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("code-reviewer")
	svc := employee.NewService(repo, roles, nil)
	reg := employee.NewRegistry(svc)

	projectID := uuid.New()
	ctx := context.Background()

	report, err := reg.SeedFromDir(ctx, dir, []uuid.UUID{projectID})
	if err != nil {
		t.Fatalf("SeedFromDir error: %v", err)
	}
	if report.Upserted != 1 {
		t.Fatalf("expected Upserted=1, got %d (errors: %v)", report.Upserted, report.Errors)
	}

	// Find the created employee and verify its skills via svc.Get (which hydrates Skills).
	employees, err := svc.ListByProject(ctx, projectID, repository.EmployeeFilter{})
	if err != nil || len(employees) != 1 {
		t.Fatalf("expected 1 employee, got %d (err: %v)", len(employees), err)
	}

	got, err := svc.Get(ctx, employees[0].ID)
	if err != nil {
		t.Fatalf("Get employee: %v", err)
	}
	if len(got.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d: %v", len(got.Skills), got.Skills)
	}

	skillPaths := map[string]bool{}
	for _, sk := range got.Skills {
		skillPaths[sk.SkillPath] = true
	}
	if !skillPaths["skills/typescript"] {
		t.Error("missing skills/typescript")
	}
	if !skillPaths["skills/go"] {
		t.Error("missing skills/go")
	}
}
