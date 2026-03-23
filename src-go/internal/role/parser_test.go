package role_test

import (
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

func TestParse(t *testing.T) {
	manifest, err := role.Parse([]byte(validRoleManifest))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if manifest.Metadata.Name != "reviewer" {
		t.Errorf("Metadata.Name = %q, want reviewer", manifest.Metadata.Name)
	}
	if manifest.Capabilities.MaxConcurrency != 2 {
		t.Errorf("Capabilities.MaxConcurrency = %d, want 2", manifest.Capabilities.MaxConcurrency)
	}
	if !manifest.Security.RequireReview {
		t.Error("Security.RequireReview = false, want true")
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
