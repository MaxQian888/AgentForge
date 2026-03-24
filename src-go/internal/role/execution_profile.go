package role

func BuildExecutionProfile(manifest *Manifest) *ExecutionProfile {
	if manifest == nil {
		return nil
	}

	normalized := cloneManifest(manifest)
	if err := finalizeRoleManifest(normalized); err != nil {
		return nil
	}

	return &ExecutionProfile{
		RoleID:         normalized.Metadata.ID,
		Name:           normalized.Metadata.Name,
		Role:           firstNonEmpty(normalized.Identity.Role, normalized.Metadata.Name),
		Goal:           normalized.Identity.Goal,
		Backstory:      normalized.Identity.Backstory,
		SystemPrompt:   normalized.SystemPrompt,
		AllowedTools:   append([]string(nil), normalized.Capabilities.AllowedTools...),
		MaxBudgetUsd:   normalized.Security.MaxBudgetUsd,
		MaxTurns:       normalized.Capabilities.MaxTurns,
		PermissionMode: normalized.Security.PermissionMode,
	}
}
