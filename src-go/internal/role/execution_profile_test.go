package role_test

import (
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
