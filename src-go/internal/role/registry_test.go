package role_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/react-go-quick-starter/server/internal/role"
)

func TestRegistryLoadDirSkipsInvalidFilesAndSupportsLookup(t *testing.T) {
	dir := t.TempDir()
	writeRoleFile(t, dir, "reviewer.yaml", validRoleManifest)
	writeRoleFile(t, dir, "planner.yml", stringsReplace(validRoleManifest, "reviewer", "planner"))
	writeRoleFile(t, dir, "invalid.yaml", "metadata: [")
	writeRoleFile(t, dir, "empty-name.yaml", stringsReplace(validRoleManifest, "name: reviewer", "name:"))
	writeRoleFile(t, dir, "notes.txt", validRoleManifest)

	registry := role.NewRegistry()
	if err := registry.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	if got := registry.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}

	reviewer, ok := registry.Get("reviewer")
	if !ok {
		t.Fatal("Get(reviewer) ok = false, want true")
	}
	if reviewer.Metadata.Description != "Reviews code changes" {
		t.Errorf("reviewer description = %q", reviewer.Metadata.Description)
	}

	names := registry.List()
	slices.Sort(names)
	if !slices.Equal(names, []string{"planner", "reviewer"}) {
		t.Fatalf("List() = %v, want [planner reviewer]", names)
	}

	all := registry.All()
	delete(all, "reviewer")
	if registry.Count() != 2 {
		t.Fatalf("All() should return a copied map; Count() = %d", registry.Count())
	}
}

func TestRegistryLoadDirPrefersCanonicalRoleLayout(t *testing.T) {
	dir := t.TempDir()
	writeRoleFile(t, dir, "frontend-developer.yaml", legacyRoleManifest)
	writeRoleFile(t, filepath.Join(dir, "frontend-developer"), "role.yaml", canonicalRoleManifest)

	registry := role.NewRegistry()
	if err := registry.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	loaded, ok := registry.Get("frontend-developer")
	if !ok {
		t.Fatal("Get(frontend-developer) ok = false, want true")
	}
	if loaded.Metadata.Name != "Frontend Developer" {
		t.Fatalf("Metadata.Name = %q, want canonical role name", loaded.Metadata.Name)
	}
}

func TestRegistryLoadDirResolvesInheritanceWithStricterSecurity(t *testing.T) {
	dir := t.TempDir()
	writeRoleFile(t, filepath.Join(dir, "base-developer"), "role.yaml", `
apiVersion: agentforge/v1
kind: Role
metadata:
  id: base-developer
  name: Base Developer
  version: "1.0.0"
identity:
  role: Base Developer
  goal: Build safely
  backstory: Base role
security:
  permission_mode: bypassPermissions
  allowed_paths: ["src/", "app/"]
  max_budget_usd: 10
capabilities:
  skills:
    - path: skills/react
      auto_load: true
    - path: skills/typescript
      auto_load: false
`)
	writeRoleFile(t, filepath.Join(dir, "frontend-developer"), "role.yaml", `
apiVersion: agentforge/v1
kind: Role
metadata:
  id: frontend-developer
  name: Frontend Developer
  version: "1.0.0"
extends: base-developer
identity:
  role: Frontend Developer
  goal: Build UI safely
security:
  permission_mode: default
  allowed_paths: ["app/"]
  max_budget_usd: 5
capabilities:
  skills:
    - path: skills/react
      auto_load: false
    - path: skills/testing
      auto_load: true
`)

	registry := role.NewRegistry()
	if err := registry.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	loaded, ok := registry.Get("frontend-developer")
	if !ok {
		t.Fatal("Get(frontend-developer) ok = false, want true")
	}
	if loaded.Identity.Backstory != "Base role" {
		t.Fatalf("Backstory = %q, want inherited backstory", loaded.Identity.Backstory)
	}
	if loaded.Security.PermissionMode != "default" {
		t.Fatalf("PermissionMode = %q, want stricter child value", loaded.Security.PermissionMode)
	}
	if loaded.Security.MaxBudgetUsd != 5 {
		t.Fatalf("MaxBudgetUsd = %v, want 5", loaded.Security.MaxBudgetUsd)
	}
	if !slices.Equal(loaded.Security.AllowedPaths, []string{"app/"}) {
		t.Fatalf("AllowedPaths = %v, want stricter child allowed_paths", loaded.Security.AllowedPaths)
	}

	payload, err := json.Marshal(loaded)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	capabilities := decoded["capabilities"].(map[string]any)
	skills := capabilities["skills"].([]any)
	if len(skills) != 3 {
		t.Fatalf("resolved skills len = %d, want 3", len(skills))
	}
	first := skills[0].(map[string]any)
	if first["path"] != "skills/react" || first["autoLoad"] != false {
		t.Fatalf("first resolved skill = %#v, want child override for skills/react", first)
	}
	second := skills[1].(map[string]any)
	if second["path"] != "skills/typescript" {
		t.Fatalf("second resolved skill = %#v, want inherited skills/typescript", second)
	}
	third := skills[2].(map[string]any)
	if third["path"] != "skills/testing" || third["autoLoad"] != true {
		t.Fatalf("third resolved skill = %#v, want appended skills/testing", third)
	}
}

func TestRegistryLoadDirMergesAdvancedAuthoringSections(t *testing.T) {
	dir := t.TempDir()
	writeRoleFile(t, filepath.Join(dir, "base-role"), "role.yaml", `
apiVersion: agentforge/v1
kind: Role
metadata:
  id: base-role
  name: Base Role
identity:
  role: Base Role
capabilities:
  packages: [shared]
  tools:
    built_in: [Read]
knowledge:
  shared:
    - id: design-guidelines
      type: vector
      access: read
security:
  profile: development
  allowed_paths: ["src/", "app/"]
  output_filters: [no_pii]
collaboration:
  can_delegate_to: [frontend-developer]
triggers:
  - event: pr_created
    action: notify
`)
	writeRoleFile(t, filepath.Join(dir, "child-role"), "role.yaml", `
apiVersion: agentforge/v1
kind: Role
metadata:
  id: child-role
  name: Child Role
extends: base-role
identity:
  role: Child Role
capabilities:
  packages: [review]
  tools:
    external: [figma]
knowledge:
  shared:
    - id: child-guidelines
      type: vector
      access: read
security:
  profile: standard
  allowed_paths: ["app/"]
  output_filters: [no_credentials]
collaboration:
  accepts_delegation_from: [design-manager]
triggers:
  - event: pr_created
    action: auto_review
`)

	registry := role.NewRegistry()
	if err := registry.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	loaded, ok := registry.Get("child-role")
	if !ok {
		t.Fatal("Get(child-role) ok = false, want true")
	}
	if got := loaded.Capabilities.Packages; len(got) != 2 || got[0] != "shared" || got[1] != "review" {
		t.Fatalf("Capabilities.Packages = %v, want inherited + child package merge", got)
	}
	if got := loaded.Knowledge.Shared; len(got) != 2 {
		t.Fatalf("Knowledge.Shared = %#v, want merged shared knowledge entries", got)
	}
	if got := loaded.Security.OutputFilters; len(got) != 2 {
		t.Fatalf("Security.OutputFilters = %v, want merged output filters", got)
	}
	if loaded.Collaboration.AcceptsDelegationFrom[0] != "design-manager" {
		t.Fatalf("AcceptsDelegationFrom = %v, want child collaboration field", loaded.Collaboration.AcceptsDelegationFrom)
	}
	if got := loaded.Triggers; len(got) != 2 {
		t.Fatalf("Triggers = %#v, want merged trigger list", got)
	}
}

func TestRegistryRegisterAndLoadDirErrors(t *testing.T) {
	registry := role.NewRegistry()
	registry.Register(&role.Manifest{
		Metadata: role.Metadata{ID: "custom", Name: "custom"},
	})

	if got := registry.Count(); got != 1 {
		t.Fatalf("Count() after Register = %d, want 1", got)
	}
	if _, ok := registry.Get("custom"); !ok {
		t.Fatal("Get(custom) ok = false, want true")
	}

	if err := registry.LoadDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("LoadDir() error = nil, want read failure")
	}
}

func writeRoleFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", name, err)
	}
}

func stringsReplace(input, old, new string) string {
	return strings.NewReplacer(old, new).Replace(input)
}
