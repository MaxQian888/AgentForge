package role

import "testing"

func TestBuildRoleCapabilitySetNormalizesLegacyToolsAndRoleHints(t *testing.T) {
	manifest := &Manifest{}
	manifest.Capabilities.AllowedTools = []string{"Read", "Edit", "Bash"}
	manifest.Capabilities.Packages = []string{"web-development"}
	manifest.Capabilities.Frameworks = []string{"Next.js"}
	manifest.Capabilities.ToolConfig.External = []string{"figma"}
	manifest.Capabilities.ToolConfig.MCPServers = []RoleMCPServer{{Name: "design-mcp"}}

	got := buildRoleCapabilitySet(manifest)

	for _, capability := range []string{
		"read",
		"edit",
		"bash",
		"code_editor",
		"terminal",
		"browser_preview",
		"web_development",
		"next_js",
		"figma",
		"design_mcp",
	} {
		if _, ok := got[capability]; !ok {
			t.Fatalf("buildRoleCapabilitySet() missing %q in %v", capability, cloneCapabilitySet(got))
		}
	}
}

func TestMissingRequiredCapabilitiesReturnsNormalizedMissingValues(t *testing.T) {
	available := map[string]struct{}{
		"code_editor": {},
		"terminal":    {},
	}

	got := missingRequiredCapabilities([]string{"code-editor", "browser preview", "terminal"}, available)
	if len(got) != 1 || got[0] != "browser_preview" {
		t.Fatalf("missingRequiredCapabilities() = %v, want [browser_preview]", got)
	}
}
