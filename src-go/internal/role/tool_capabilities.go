package role

import (
	"maps"
	"slices"
	"strings"
)

func buildRoleCapabilitySet(manifest *Manifest) map[string]struct{} {
	capabilities := make(map[string]struct{})
	if manifest == nil {
		return capabilities
	}

	addRoleCapabilityValues(capabilities, manifest.Capabilities.AllowedTools...)
	addRoleCapabilityValues(capabilities, manifest.Capabilities.Tools...)
	addRoleCapabilityValues(capabilities, manifest.Capabilities.ToolConfig.BuiltIn...)
	addRoleCapabilityValues(capabilities, manifest.Capabilities.ToolConfig.External...)

	for _, server := range manifest.Capabilities.ToolConfig.MCPServers {
		addRoleCapabilityValues(capabilities, server.Name)
	}
	addRoleCapabilityValues(capabilities, manifest.Capabilities.Packages...)
	addRoleCapabilityValues(capabilities, manifest.Capabilities.Frameworks...)

	return capabilities
}

func addRoleCapabilityValues(target map[string]struct{}, values ...string) {
	for _, value := range values {
		token := normalizeCapabilityToken(value)
		if token == "" {
			continue
		}
		target[token] = struct{}{}
		for _, alias := range roleCapabilityAliases(token) {
			target[alias] = struct{}{}
		}
	}
}

func roleCapabilityAliases(token string) []string {
	switch token {
	case "read", "edit", "write", "glob", "grep", "multiedit":
		return []string{"code_editor"}
	case "bash", "terminal", "terminal_access":
		return []string{"terminal"}
	case "browser_preview":
		return []string{"browser_preview"}
	case "web_development":
		return []string{"code_editor", "terminal", "browser_preview"}
	case "testing":
		return []string{"code_editor", "terminal"}
	case "design_system", "react", "next_js", "nextjs", "vue", "vue_js", "svelte":
		return []string{"browser_preview"}
	default:
		return nil
	}
}

func normalizeCapabilityToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "_", "-", "_", ".", "_", "/", "_")
	return replacer.Replace(value)
}

func normalizeCapabilityTokens(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		token := normalizeCapabilityToken(value)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		normalized = append(normalized, token)
	}
	return normalized
}

func missingRequiredCapabilities(required []string, available map[string]struct{}) []string {
	if len(required) == 0 {
		return nil
	}

	missing := make([]string, 0, len(required))
	seen := make(map[string]struct{})
	for _, token := range required {
		token = normalizeCapabilityToken(token)
		if token == "" {
			continue
		}
		if _, ok := available[token]; ok {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		missing = append(missing, token)
	}
	return missing
}

func cloneCapabilitySet(values map[string]struct{}) []string {
	cloned := slices.Collect(maps.Keys(values))
	slices.Sort(cloned)
	return cloned
}
