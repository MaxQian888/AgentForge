package role

import (
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
)

func BuildExecutionProfile(manifest *Manifest, opts ...ExecutionProfileOption) *ExecutionProfile {
	if manifest == nil {
		return nil
	}

	normalized := cloneManifest(manifest)
	if err := finalizeRoleManifest(normalized); err != nil {
		return nil
	}

	options := executionProfileOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	loadedSkills, availableSkills, skillDiagnostics := resolveRuntimeSkills(normalized, options.skillRoot)
	skillDiagnostics = appendSkillCompatibilityDiagnostics(normalized, loadedSkills, availableSkills, skillDiagnostics)

	return &ExecutionProfile{
		RoleID:           normalized.Metadata.ID,
		Name:             normalized.Metadata.Name,
		Role:             firstNonEmpty(normalized.Identity.Role, normalized.Metadata.Name),
		Goal:             normalized.Identity.Goal,
		Backstory:        normalized.Identity.Backstory,
		SystemPrompt:     normalized.SystemPrompt,
		AllowedTools:     append([]string(nil), normalized.Capabilities.AllowedTools...),
		Tools:            buildExecutionToolIDs(normalized),
		PluginBindings:   append([]model.RoleToolPluginBinding(nil), normalized.Capabilities.ToolConfig.PluginBindings...),
		KnowledgeContext: buildKnowledgeContext(normalized),
		OutputFilters:    append([]string(nil), normalized.Security.OutputFilters...),
		MaxBudgetUsd:     normalized.Security.MaxBudgetUsd,
		MaxTurns:         normalized.Capabilities.MaxTurns,
		PermissionMode:   normalized.Security.PermissionMode,
		LoadedSkills:     loadedSkills,
		AvailableSkills:  availableSkills,
		SkillDiagnostics: skillDiagnostics,
	}
}

func buildExecutionToolIDs(manifest *Manifest) []string {
	if manifest == nil {
		return nil
	}

	seen := make(map[string]struct{})
	ids := make([]string, 0, len(manifest.Capabilities.ToolConfig.External)+len(manifest.Capabilities.ToolConfig.MCPServers))
	for _, value := range manifest.Capabilities.ToolConfig.External {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	for _, server := range manifest.Capabilities.ToolConfig.MCPServers {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		ids = append(ids, name)
	}
	return ids
}

func buildKnowledgeContext(manifest *Manifest) string {
	if manifest == nil {
		return ""
	}

	parts := make([]string, 0, 5)
	if len(manifest.Knowledge.Repositories) > 0 {
		parts = append(parts, "Repositories: "+strings.Join(manifest.Knowledge.Repositories, ", "))
	}
	if len(manifest.Knowledge.Documents) > 0 {
		parts = append(parts, "Documents: "+strings.Join(manifest.Knowledge.Documents, ", "))
	}
	if len(manifest.Knowledge.Patterns) > 0 {
		parts = append(parts, "Patterns: "+strings.Join(manifest.Knowledge.Patterns, ", "))
	}
	if len(manifest.Knowledge.Shared) > 0 {
		shared := make([]string, 0, len(manifest.Knowledge.Shared))
		for _, source := range manifest.Knowledge.Shared {
			if value := strings.TrimSpace(source.ID); value != "" {
				shared = append(shared, value)
			}
		}
		if len(shared) > 0 {
			parts = append(parts, "Shared Sources: "+strings.Join(shared, ", "))
		}
	}
	if len(manifest.Knowledge.Private) > 0 {
		private := make([]string, 0, len(manifest.Knowledge.Private))
		for _, source := range manifest.Knowledge.Private {
			if value := strings.TrimSpace(source.ID); value != "" {
				private = append(private, value)
			}
		}
		if len(private) > 0 {
			parts = append(parts, "Private Sources: "+strings.Join(private, ", "))
		}
	}
	return strings.Join(parts, "\n")
}
