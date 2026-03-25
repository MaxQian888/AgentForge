package role_test

import (
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
