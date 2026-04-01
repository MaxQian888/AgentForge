package role_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/react-go-quick-starter/server/internal/role"
)

func TestBuildExecutionProfileUsesResolvedRoleShape(t *testing.T) {
	manifest, err := role.Parse([]byte(canonicalRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	profile := role.BuildExecutionProfile(manifest)
	if profile.RoleID != "frontend-developer" {
		t.Fatalf("RoleID = %q, want frontend-developer", profile.RoleID)
	}
	if profile.Role != "Senior Frontend Developer" {
		t.Fatalf("Role = %q, want Senior Frontend Developer", profile.Role)
	}
	if profile.SystemPrompt == "" {
		t.Fatal("SystemPrompt = empty, want synthesized prompt")
	}
	if len(profile.AllowedTools) != 3 {
		t.Fatalf("AllowedTools = %v, want normalized built_in tools", profile.AllowedTools)
	}
	if profile.PermissionMode != "default" {
		t.Fatalf("PermissionMode = %q, want default", profile.PermissionMode)
	}
}

func TestBuildExecutionProfileProjectsRuntimeFacingAdvancedRoleFields(t *testing.T) {
	manifest, err := role.Parse([]byte(advancedRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	profile := role.BuildExecutionProfile(manifest)
	assertExecutionProfileStringSlice(t, profile, "Tools", []string{"figma", "design-mcp"})
	assertExecutionProfileStringSlice(t, profile, "OutputFilters", []string{"no_pii", "no_credentials"})

	knowledgeContext := assertExecutionProfileStringField(t, profile, "KnowledgeContext")
	if !strings.Contains(knowledgeContext, "docs/PRD.md") {
		t.Fatalf("KnowledgeContext = %q, want docs/PRD.md reference", knowledgeContext)
	}
	if !strings.Contains(knowledgeContext, "design-guidelines") {
		t.Fatalf("KnowledgeContext = %q, want shared knowledge source", knowledgeContext)
	}
}

func TestBuildExecutionProfileProjectsLoadedAndAvailableSkills(t *testing.T) {
	skillsDir := t.TempDir()
	mustWriteSkillFixture(t, filepath.Join(skillsDir, "react", "SKILL.md"), `---
name: React
description: React UI implementation guidance
requires:
  - skills/typescript
tools:
  - code_editor
  - browser_preview
---

# React

Prefer server-safe React and repository-aligned component structure.
`)
	mustWriteSkillFixture(t, filepath.Join(skillsDir, "react", "agents", "openai.yaml"), `interface:
  display_name: "React Workspace"
  short_description: "Guide React work in this repository."
  default_prompt: "Use $react to implement the current React seam."
`)
	mustWriteSkillFixture(t, filepath.Join(skillsDir, "react", "references", "surface-map.md"), `# Surface Map`)
	mustWriteSkillFixture(t, filepath.Join(skillsDir, "typescript", "SKILL.md"), `---
name: TypeScript
description: Type-safe contracts and refactors
tools:
  - code_editor
  - terminal
--- 

# TypeScript

Prefer explicit contracts and narrow public surfaces.
`)
	mustWriteSkillFixture(t, filepath.Join(skillsDir, "testing", "SKILL.md"), `---
name: Testing
description: Regression-oriented test guidance
tools:
  - code_editor
  - terminal
---

# Testing

Write targeted regression coverage before broad refactors.
`)

	manifest, err := role.Parse([]byte(canonicalRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	profile := role.BuildExecutionProfile(manifest, role.WithSkillRoot(skillsDir))
	loadedSkills := assertExecutionProfileStructSlice(t, profile, "LoadedSkills")
	if len(loadedSkills) != 2 {
		t.Fatalf("LoadedSkills len = %d, want 2", len(loadedSkills))
	}
	if loadedSkills[0].FieldByName("Path").String() != "skills/react" {
		t.Fatalf("LoadedSkills[0].Path = %q, want skills/react", loadedSkills[0].FieldByName("Path").String())
	}
	if loadedSkills[1].FieldByName("Path").String() != "skills/typescript" {
		t.Fatalf("LoadedSkills[1].Path = %q, want skills/typescript", loadedSkills[1].FieldByName("Path").String())
	}
	if loadedSkills[0].FieldByName("Instructions").String() == "" {
		t.Fatal("LoadedSkills[0].Instructions = empty, want injected skill instructions")
	}
	if loadedSkills[0].FieldByName("DisplayName").String() != "React Workspace" {
		t.Fatalf("LoadedSkills[0].DisplayName = %q, want interface display name", loadedSkills[0].FieldByName("DisplayName").String())
	}
	if loadedSkills[0].FieldByName("ShortDescription").String() != "Guide React work in this repository." {
		t.Fatalf("LoadedSkills[0].ShortDescription = %q, want interface short description", loadedSkills[0].FieldByName("ShortDescription").String())
	}
	if loadedSkills[0].FieldByName("DefaultPrompt").String() == "" {
		t.Fatal("LoadedSkills[0].DefaultPrompt = empty, want interface default prompt")
	}
	parts := loadedSkills[0].FieldByName("AvailableParts")
	if parts.Len() != 2 || parts.Index(0).String() != "agents" || parts.Index(1).String() != "references" {
		t.Fatalf("LoadedSkills[0].AvailableParts = %v, want agents and references", parts)
	}
	requires := loadedSkills[0].FieldByName("Requires")
	if requires.Len() != 1 || requires.Index(0).String() != "skills/typescript" {
		t.Fatalf("LoadedSkills[0].Requires = %v, want skills/typescript", requires)
	}
	declaredTools := loadedSkills[0].FieldByName("Tools")
	if declaredTools.Len() != 2 || declaredTools.Index(0).String() != "code_editor" || declaredTools.Index(1).String() != "browser_preview" {
		t.Fatalf("LoadedSkills[0].Tools = %v, want normalized tool requirements", declaredTools)
	}
	availableSkills := assertExecutionProfileStructSlice(t, profile, "AvailableSkills")
	if len(availableSkills) != 1 {
		t.Fatalf("AvailableSkills len = %d, want 1", len(availableSkills))
	}
	if availableSkills[0].FieldByName("Path").String() != "skills/testing" {
		t.Fatalf("AvailableSkills[0].Path = %q, want skills/testing", availableSkills[0].FieldByName("Path").String())
	}
	if availableSkills[0].FieldByName("Instructions").String() != "" {
		t.Fatalf("AvailableSkills[0].Instructions = %q, want empty on-demand inventory", availableSkills[0].FieldByName("Instructions").String())
	}
	availableTools := availableSkills[0].FieldByName("Tools")
	if availableTools.Len() != 2 || availableTools.Index(0).String() != "code_editor" || availableTools.Index(1).String() != "terminal" {
		t.Fatalf("AvailableSkills[0].Tools = %v, want normalized tool requirements", availableTools)
	}
	if diagnostics := assertExecutionProfileStructSlice(t, profile, "SkillDiagnostics"); len(diagnostics) != 0 {
		t.Fatalf("SkillDiagnostics len = %d, want 0", len(diagnostics))
	}
}

func TestBuildExecutionProfileReportsBlockingDiagnosticsForMissingAutoLoadSkills(t *testing.T) {
	skillsDir := t.TempDir()
	mustWriteSkillFixture(t, filepath.Join(skillsDir, "testing", "SKILL.md"), `---
name: Testing
description: Regression-oriented test guidance
---

# Testing
`)

	manifest, err := role.Parse([]byte(canonicalRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	profile := role.BuildExecutionProfile(manifest, role.WithSkillRoot(skillsDir))
	diagnostics := assertExecutionProfileStructSlice(t, profile, "SkillDiagnostics")
	if len(diagnostics) != 1 {
		t.Fatalf("SkillDiagnostics len = %d, want 1", len(diagnostics))
	}
	if diagnostics[0].FieldByName("Blocking").Bool() != true {
		t.Fatalf("SkillDiagnostics[0].Blocking = %v, want true", diagnostics[0].FieldByName("Blocking").Bool())
	}
	if diagnostics[0].FieldByName("Path").String() != "skills/react" {
		t.Fatalf("SkillDiagnostics[0].Path = %q, want skills/react", diagnostics[0].FieldByName("Path").String())
	}
}

func TestBuildExecutionProfileReportsBlockingDiagnosticsForMissingAutoLoadToolCompatibility(t *testing.T) {
	skillsDir := t.TempDir()
	mustWriteSkillFixture(t, filepath.Join(skillsDir, "react", "SKILL.md"), `---
name: React
tools:
  - browser_preview
---

# React
`)

	manifest, err := role.Parse([]byte(`apiVersion: agentforge/v1
kind: Role
metadata:
  id: ui-role
  name: UI Role
identity:
  role: UI Role
  goal: Build UI
capabilities:
  tools:
    built_in: [Read, Edit]
  skills:
    - path: skills/react
      auto_load: true
security:
  permission_mode: default
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	profile := role.BuildExecutionProfile(manifest, role.WithSkillRoot(skillsDir))
	diagnostics := assertExecutionProfileStructSlice(t, profile, "SkillDiagnostics")
	if len(diagnostics) != 1 {
		t.Fatalf("SkillDiagnostics len = %d, want 1", len(diagnostics))
	}
	if diagnostics[0].FieldByName("Code").String() != "role_skill_tools_unavailable" {
		t.Fatalf("SkillDiagnostics[0].Code = %q, want role_skill_tools_unavailable", diagnostics[0].FieldByName("Code").String())
	}
	if !diagnostics[0].FieldByName("Blocking").Bool() {
		t.Fatalf("SkillDiagnostics[0].Blocking = %v, want true", diagnostics[0].FieldByName("Blocking").Bool())
	}
	if !strings.Contains(diagnostics[0].FieldByName("Message").String(), "browser_preview") {
		t.Fatalf("SkillDiagnostics[0].Message = %q, want missing browser_preview detail", diagnostics[0].FieldByName("Message").String())
	}
}

func TestBuildExecutionProfileReportsWarningDiagnosticsForOnDemandToolCompatibility(t *testing.T) {
	skillsDir := t.TempDir()
	mustWriteSkillFixture(t, filepath.Join(skillsDir, "testing", "SKILL.md"), `---
name: Testing
tools:
  - terminal
---

# Testing
`)

	manifest, err := role.Parse([]byte(`apiVersion: agentforge/v1
kind: Role
metadata:
  id: review-role
  name: Review Role
identity:
  role: Review Role
  goal: Review code
capabilities:
  tools:
    built_in: [Read, Edit]
  skills:
    - path: skills/testing
      auto_load: false
security:
  permission_mode: default
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	profile := role.BuildExecutionProfile(manifest, role.WithSkillRoot(skillsDir))
	diagnostics := assertExecutionProfileStructSlice(t, profile, "SkillDiagnostics")
	if len(diagnostics) != 1 {
		t.Fatalf("SkillDiagnostics len = %d, want 1", len(diagnostics))
	}
	if diagnostics[0].FieldByName("Code").String() != "role_skill_tools_unavailable" {
		t.Fatalf("SkillDiagnostics[0].Code = %q, want role_skill_tools_unavailable", diagnostics[0].FieldByName("Code").String())
	}
	if diagnostics[0].FieldByName("Blocking").Bool() {
		t.Fatalf("SkillDiagnostics[0].Blocking = %v, want false", diagnostics[0].FieldByName("Blocking").Bool())
	}
	if !strings.Contains(diagnostics[0].FieldByName("Message").String(), "terminal") {
		t.Fatalf("SkillDiagnostics[0].Message = %q, want missing terminal detail", diagnostics[0].FieldByName("Message").String())
	}
}

func assertExecutionProfileStringSlice(t *testing.T, profile any, fieldName string, want []string) {
	t.Helper()
	rv := reflect.ValueOf(profile)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	field := rv.FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("expected field %s on execution profile", fieldName)
	}
	got := make([]string, field.Len())
	for i := 0; i < field.Len(); i++ {
		got[i] = field.Index(i).String()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", fieldName, got, want)
	}
}

func assertExecutionProfileStringField(t *testing.T, profile any, fieldName string) string {
	t.Helper()
	rv := reflect.ValueOf(profile)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	field := rv.FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("expected field %s on execution profile", fieldName)
	}
	return field.String()
}

func assertExecutionProfileStructSlice(t *testing.T, profile any, fieldName string) []reflect.Value {
	t.Helper()
	rv := reflect.ValueOf(profile)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	field := rv.FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("expected field %s on execution profile", fieldName)
	}
	values := make([]reflect.Value, 0, field.Len())
	for i := 0; i < field.Len(); i++ {
		values = append(values, field.Index(i))
	}
	return values
}

func mustWriteSkillFixture(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
