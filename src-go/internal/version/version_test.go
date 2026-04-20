package version_test

import (
	"testing"

	"github.com/agentforge/server/internal/version"
)

func TestDefaultValues(t *testing.T) {
	if version.Version != "dev" {
		t.Errorf("expected default Version 'dev', got %q", version.Version)
	}
	if version.Commit != "unknown" {
		t.Errorf("expected default Commit 'unknown', got %q", version.Commit)
	}
	if version.BuildDate != "unknown" {
		t.Errorf("expected default BuildDate 'unknown', got %q", version.BuildDate)
	}
}
