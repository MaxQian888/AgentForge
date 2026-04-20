package role_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/role"
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

const advancedRoleManifest = `
apiVersion: agentforge/v1
kind: Role
metadata:
  id: design-lead
  name: Design Lead
  version: 2.0.0
  author: AgentForge
  icon: palette
  tags: [design, lead]
identity:
  role: Senior Design Lead
  goal: Keep product UX coherent
  backstory: You coordinate design systems and interaction quality.
  persona: Collaborative design lead
  goals: [Improve polish, align patterns]
  constraints: [Do not invent unsupported tokens]
  personality: patient
  language: zh-CN
  response_style:
    tone: professional
    verbosity: concise
    format_preference: markdown
capabilities:
  packages: [design-system, review]
  tools:
    built_in: [Read, Edit]
    external: [figma]
    mcp_servers:
      - name: design-mcp
        url: http://localhost:3010/mcp
  custom_settings:
    approval_mode: guided
knowledge:
  repositories: [agentforge]
  documents: [docs/PRD.md]
  patterns: [design-system]
  shared:
    - id: design-guidelines
      type: vector
      access: read
      description: Shared design guidelines
  private:
    - id: ux-notes
      type: vector
      sources: [knowledge/ux-notes.md]
  memory:
    short_term:
      max_tokens: 64000
    episodic:
      enabled: true
      retention_days: 45
security:
  profile: standard
  permission_mode: default
  allowed_paths: [app/, components/]
  permissions:
    file_access:
      allowed_paths: [app/, components/]
      denied_paths: [secrets/]
    network:
      allowed_domains: [figma.com]
    code_execution:
      sandbox: true
      allowed_languages: [typescript]
  output_filters: [no_pii, no_credentials]
  resource_limits:
    token_budget:
      per_task: 50000
    api_calls:
      per_minute: 5
    execution_time:
      per_task: 30m
    cost_limit:
      per_task: "$5"
collaboration:
  can_delegate_to: [frontend-developer]
  accepts_delegation_from: [product-manager]
  communication:
    preferred_channel: structured
    report_format: markdown
triggers:
  - event: pr_created
    action: auto_review
    condition: "labels.includes('design')"
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

func TestParseAdvancedManifestPreservesStructuredAuthoringFields(t *testing.T) {
	manifest, err := role.Parse([]byte(advancedRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if manifest.Metadata.Icon != "palette" {
		t.Fatalf("Metadata.Icon = %q, want palette", manifest.Metadata.Icon)
	}
	if manifest.Identity.ResponseStyle.FormatPreference != "markdown" {
		t.Fatalf("ResponseStyle.FormatPreference = %q, want markdown", manifest.Identity.ResponseStyle.FormatPreference)
	}
	if got := manifest.Capabilities.ToolConfig.External; len(got) != 1 || got[0] != "figma" {
		t.Fatalf("ToolConfig.External = %v, want [figma]", got)
	}
	if got := manifest.Knowledge.Shared; len(got) != 1 || got[0].ID != "design-guidelines" {
		t.Fatalf("Knowledge.Shared = %#v, want shared knowledge entry", got)
	}
	if !manifest.Security.Permissions.CodeExecution.Sandbox {
		t.Fatal("Security.Permissions.CodeExecution.Sandbox = false, want true")
	}
	if got := manifest.Security.OutputFilters; len(got) != 2 || got[0] != "no_pii" {
		t.Fatalf("Security.OutputFilters = %v, want preserved filters", got)
	}
	if manifest.Collaboration.Communication.PreferredChannel != "structured" {
		t.Fatalf("PreferredChannel = %q, want structured", manifest.Collaboration.Communication.PreferredChannel)
	}
	if got := manifest.Triggers; len(got) != 1 || got[0].Action != "auto_review" {
		t.Fatalf("Triggers = %#v, want trigger action auto_review", got)
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
