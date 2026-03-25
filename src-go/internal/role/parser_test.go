package role_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/react-go-quick-starter/server/internal/role"
)

const validRoleManifest = `
metadata:
  name: reviewer
  version: 1.0.0
  description: Reviews code changes
  author: AgentForge
  tags: [review, qa]
identity:
  system_prompt: Review carefully
  persona: Reviewer
  goals: [Catch bugs]
  constraints: [No silent failures]
capabilities:
  tools: [read_file, search]
  languages: [go, typescript]
  frameworks: [echo, nextjs]
  max_concurrency: 2
  custom_settings:
    mode: strict
knowledge:
  repositories: [agentforge]
  documents: [README.md]
  patterns: [service-repository]
security:
  allowed_paths: [src-go]
  denied_paths: [secrets]
  max_budget_usd: 5
  require_review: true
`

const canonicalRoleManifest = `
apiVersion: agentforge/v1
kind: Role
metadata:
  id: frontend-developer
  name: Frontend Developer
  version: 1.2.0
  author: team-admin
  tags: [development, frontend]
  description: Frontend specialist
identity:
  role: Senior Frontend Developer
  goal: Build reliable UI
  backstory: You build maintainable frontend systems.
capabilities:
  packages: [web-development]
  tools:
    built_in: [Read, Edit, Bash]
  skills:
    - path: skills/react
      auto_load: true
    - path: skills/testing
      auto_load: false
  max_concurrency: 2
security:
  permission_mode: default
  allowed_paths: [app/, components/]
`

const legacyRoleManifest = `
metadata:
  name: frontend-developer
  version: "1.0"
  description: "Frontend development specialist for React and Next.js"
  tags: [frontend, react, nextjs]
identity:
  goal: "Build responsive, accessible, and performant UI components and pages"
  backstory: "You are a frontend engineer expert in React, Next.js, Tailwind CSS, and modern web standards"
capabilities:
  allowed_tools: [Read, Edit, Write, Bash, Glob, Grep]
  max_turns: 30
  max_budget_usd: 5.0
knowledge:
  system_prompt: |
    You are a frontend developer.
security:
  permission_mode: "bypassPermissions"
  allowed_paths: ["app/", "components/"]
`

func TestParse(t *testing.T) {
	manifest, err := role.Parse([]byte(validRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if manifest.Metadata.Name != "reviewer" {
		t.Errorf("Metadata.Name = %q, want reviewer", manifest.Metadata.Name)
	}
	if manifest.APIVersion != "agentforge/v1" {
		t.Errorf("APIVersion = %q, want agentforge/v1", manifest.APIVersion)
	}
	if manifest.Kind != "Role" {
		t.Errorf("Kind = %q, want Role", manifest.Kind)
	}
	if manifest.Metadata.ID != "reviewer" {
		t.Errorf("Metadata.ID = %q, want reviewer", manifest.Metadata.ID)
	}
	if manifest.Capabilities.MaxConcurrency != 2 {
		t.Errorf("Capabilities.MaxConcurrency = %d, want 2", manifest.Capabilities.MaxConcurrency)
	}
	if manifest.SystemPrompt != "Review carefully" {
		t.Errorf("SystemPrompt = %q, want Review carefully", manifest.SystemPrompt)
	}
	if !manifest.Security.RequireReview {
		t.Error("Security.RequireReview = false, want true")
	}
}

func TestParseCanonicalManifestSupportsPRDShape(t *testing.T) {
	manifest, err := role.Parse([]byte(canonicalRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if manifest.Metadata.ID != "frontend-developer" {
		t.Fatalf("Metadata.ID = %q, want frontend-developer", manifest.Metadata.ID)
	}
	if manifest.Identity.Role != "Senior Frontend Developer" {
		t.Fatalf("Identity.Role = %q, want Senior Frontend Developer", manifest.Identity.Role)
	}
	if manifest.SystemPrompt == "" {
		t.Fatal("SystemPrompt = empty, want synthesized prompt")
	}
	if got := manifest.Capabilities.AllowedTools; len(got) != 3 || got[0] != "Read" {
		t.Fatalf("AllowedTools = %v, want built_in tools to be normalized", got)
	}

	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	capabilities := decoded["capabilities"].(map[string]any)
	skills, ok := capabilities["skills"].([]any)
	if !ok || len(skills) != 2 {
		t.Fatalf("capabilities.skills = %#v, want 2 structured skill entries", capabilities["skills"])
	}
	firstSkill := skills[0].(map[string]any)
	if firstSkill["path"] != "skills/react" {
		t.Fatalf("first skill path = %#v, want skills/react", firstSkill["path"])
	}
	if firstSkill["autoLoad"] != true {
		t.Fatalf("first skill autoLoad = %#v, want true", firstSkill["autoLoad"])
	}
}

func TestParseLegacyManifestNormalizesCurrentFlatShape(t *testing.T) {
	manifest, err := role.Parse([]byte(legacyRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if manifest.Metadata.ID != "frontend-developer" {
		t.Fatalf("Metadata.ID = %q, want frontend-developer", manifest.Metadata.ID)
	}
	if manifest.Kind != "Role" {
		t.Fatalf("Kind = %q, want Role", manifest.Kind)
	}
	if manifest.SystemPrompt != "You are a frontend developer." {
		t.Fatalf("SystemPrompt = %q, want normalized knowledge.system_prompt", manifest.SystemPrompt)
	}
	if manifest.Capabilities.MaxTurns != 30 {
		t.Fatalf("MaxTurns = %d, want 30", manifest.Capabilities.MaxTurns)
	}
	if manifest.Security.PermissionMode != "bypassPermissions" {
		t.Fatalf("PermissionMode = %q, want bypassPermissions", manifest.Security.PermissionMode)
	}
}

func TestParseRejectsInvalidYAML(t *testing.T) {
	_, err := role.Parse([]byte("metadata: ["))
	if err == nil {
		t.Fatal("Parse() error = nil, want parse failure")
	}
	if !strings.Contains(err.Error(), "parse role manifest") {
		t.Fatalf("Parse() error = %q, want wrapped parse message", err.Error())
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reviewer.yaml")
	if err := os.WriteFile(path, []byte(validRoleManifest), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manifest, err := role.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if manifest.Metadata.Author != "AgentForge" {
		t.Errorf("Metadata.Author = %q, want AgentForge", manifest.Metadata.Author)
	}
}

func TestParseFileMissingPath(t *testing.T) {
	_, err := role.ParseFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("ParseFile() error = nil, want read failure")
	}
	if !strings.Contains(err.Error(), "read role file") {
		t.Fatalf("ParseFile() error = %q, want wrapped read message", err.Error())
	}
}

func TestParseRejectsBlankOrDuplicateSkillPaths(t *testing.T) {
	_, err := role.Parse([]byte(`
apiVersion: agentforge/v1
kind: Role
metadata:
  id: broken-role
  name: Broken Role
identity:
  role: Broken Role
capabilities:
  skills:
    - path: skills/react
      auto_load: true
    - path: " "
      auto_load: false
    - path: skills/react
      auto_load: false
`))
	if err == nil {
		t.Fatal("Parse() error = nil, want invalid skills failure")
	}
	if !strings.Contains(err.Error(), "skill") {
		t.Fatalf("Parse() error = %q, want skill validation failure", err.Error())
	}
}
