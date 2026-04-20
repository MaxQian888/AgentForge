package role

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentforge/server/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	defaultAPIVersion = "agentforge/v1"
	defaultKind       = "Role"
	defaultMaxTurns   = 50
)

// ParseFile reads and parses a YAML role manifest from a file.
func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read role file %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses a YAML role manifest from bytes.
func Parse(data []byte) (*Manifest, error) {
	var raw rawRoleManifest
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse role manifest: %w", err)
	}

	manifest, err := normalizeRoleManifest(raw)
	if err != nil {
		return nil, fmt.Errorf("parse role manifest: %w", err)
	}
	return manifest, nil
}

type rawRoleManifest struct {
	APIVersion    string              `yaml:"apiVersion"`
	Kind          string              `yaml:"kind"`
	Metadata      rawRoleMetadata     `yaml:"metadata"`
	Identity      rawRoleIdentity     `yaml:"identity"`
	SystemPrompt  string              `yaml:"system_prompt"`
	Capabilities  rawRoleCapabilities `yaml:"capabilities"`
	Knowledge     rawRoleKnowledge    `yaml:"knowledge"`
	Security      rawRoleSecurity     `yaml:"security"`
	Extends       string              `yaml:"extends"`
	Overrides     map[string]any      `yaml:"overrides"`
	Collaboration RoleCollaboration   `yaml:"collaboration"`
	Triggers      []RoleTrigger       `yaml:"triggers"`
}

type rawRoleMetadata struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Author      string   `yaml:"author"`
	Tags        []string `yaml:"tags"`
	Icon        string   `yaml:"icon"`
}

type rawRoleIdentity struct {
	Role          string                  `yaml:"role"`
	Goal          string                  `yaml:"goal"`
	Backstory     string                  `yaml:"backstory"`
	SystemPrompt  string                  `yaml:"system_prompt"`
	Persona       string                  `yaml:"persona"`
	Goals         []string                `yaml:"goals"`
	Constraints   []string                `yaml:"constraints"`
	Personality   string                  `yaml:"personality"`
	Language      string                  `yaml:"language"`
	ResponseStyle model.RoleResponseStyle `yaml:"response_style"`
}

type rawRoleCapabilities struct {
	Packages       []string                   `yaml:"packages"`
	AllowedTools   []string                   `yaml:"allowed_tools"`
	Tools          yaml.Node                  `yaml:"tools"`
	Skills         []model.RoleSkillReference `yaml:"skills"`
	Languages      []string                   `yaml:"languages"`
	Frameworks     []string                   `yaml:"frameworks"`
	MaxConcurrency int                        `yaml:"max_concurrency"`
	MaxTurns       int                        `yaml:"max_turns"`
	MaxBudgetUsd   float64                    `yaml:"max_budget_usd"`
	CustomSettings map[string]string          `yaml:"custom_settings"`
}

type rawRoleKnowledge struct {
	Repositories []string              `yaml:"repositories"`
	Documents    []string              `yaml:"documents"`
	Patterns     []string              `yaml:"patterns"`
	SystemPrompt string                `yaml:"system_prompt"`
	Shared       []RoleKnowledgeSource `yaml:"shared"`
	Private      []RoleKnowledgeSource `yaml:"private"`
	Memory       RoleMemoryConfig      `yaml:"memory"`
}

type rawRoleSecurity struct {
	PermissionMode string             `yaml:"permission_mode"`
	AllowedPaths   []string           `yaml:"allowed_paths"`
	DeniedPaths    []string           `yaml:"denied_paths"`
	MaxBudgetUsd   float64            `yaml:"max_budget_usd"`
	RequireReview  bool               `yaml:"require_review"`
	Profile        string             `yaml:"profile"`
	Permissions    RolePermissions    `yaml:"permissions"`
	OutputFilters  []string           `yaml:"output_filters"`
	ResourceLimits RoleResourceLimits `yaml:"resource_limits"`
}

func normalizeRoleManifest(raw rawRoleManifest) (*Manifest, error) {
	manifest := &Manifest{
		APIVersion: firstNonEmpty(raw.APIVersion, defaultAPIVersion),
		Kind:       firstNonEmpty(raw.Kind, defaultKind),
		Metadata: model.RoleMetadata{
			ID:          strings.TrimSpace(raw.Metadata.ID),
			Name:        strings.TrimSpace(raw.Metadata.Name),
			Version:     strings.TrimSpace(raw.Metadata.Version),
			Description: strings.TrimSpace(raw.Metadata.Description),
			Author:      strings.TrimSpace(raw.Metadata.Author),
			Tags:        append([]string(nil), raw.Metadata.Tags...),
			Icon:        strings.TrimSpace(raw.Metadata.Icon),
		},
		Identity: model.RoleIdentity{
			Role:          strings.TrimSpace(raw.Identity.Role),
			Goal:          strings.TrimSpace(raw.Identity.Goal),
			Backstory:     strings.TrimSpace(raw.Identity.Backstory),
			SystemPrompt:  strings.TrimSpace(raw.Identity.SystemPrompt),
			Persona:       strings.TrimSpace(raw.Identity.Persona),
			Goals:         append([]string(nil), raw.Identity.Goals...),
			Constraints:   append([]string(nil), raw.Identity.Constraints...),
			Personality:   strings.TrimSpace(raw.Identity.Personality),
			Language:      strings.TrimSpace(raw.Identity.Language),
			ResponseStyle: raw.Identity.ResponseStyle,
		},
		SystemPrompt: strings.TrimSpace(raw.SystemPrompt),
		Capabilities: model.RoleCapabilities{
			Packages:       append([]string(nil), raw.Capabilities.Packages...),
			AllowedTools:   append([]string(nil), raw.Capabilities.AllowedTools...),
			Skills:         append([]model.RoleSkillReference(nil), raw.Capabilities.Skills...),
			Languages:      append([]string(nil), raw.Capabilities.Languages...),
			Frameworks:     append([]string(nil), raw.Capabilities.Frameworks...),
			MaxConcurrency: raw.Capabilities.MaxConcurrency,
			MaxTurns:       raw.Capabilities.MaxTurns,
			MaxBudgetUsd:   raw.Capabilities.MaxBudgetUsd,
			CustomSettings: cloneStringMap(raw.Capabilities.CustomSettings),
		},
		Knowledge: model.RoleKnowledge{
			Repositories: append([]string(nil), raw.Knowledge.Repositories...),
			Documents:    append([]string(nil), raw.Knowledge.Documents...),
			Patterns:     append([]string(nil), raw.Knowledge.Patterns...),
			SystemPrompt: strings.TrimSpace(raw.Knowledge.SystemPrompt),
			Shared:       append([]model.RoleKnowledgeSource(nil), raw.Knowledge.Shared...),
			Private:      append([]model.RoleKnowledgeSource(nil), raw.Knowledge.Private...),
			Memory:       raw.Knowledge.Memory,
		},
		Security: model.RoleSecurity{
			PermissionMode: strings.TrimSpace(raw.Security.PermissionMode),
			AllowedPaths:   append([]string(nil), raw.Security.AllowedPaths...),
			DeniedPaths:    append([]string(nil), raw.Security.DeniedPaths...),
			MaxBudgetUsd:   raw.Security.MaxBudgetUsd,
			RequireReview:  raw.Security.RequireReview,
			Profile:        strings.TrimSpace(raw.Security.Profile),
			Permissions:    raw.Security.Permissions,
			OutputFilters:  append([]string(nil), raw.Security.OutputFilters...),
			ResourceLimits: raw.Security.ResourceLimits,
		},
		Extends:       strings.TrimSpace(raw.Extends),
		Overrides:     raw.Overrides,
		Collaboration: raw.Collaboration,
		Triggers:      raw.Triggers,
	}

	if err := normalizeCapabilityTools(manifest, raw.Capabilities.Tools); err != nil {
		return nil, err
	}
	if err := finalizeRoleManifest(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func normalizeCapabilityTools(manifest *Manifest, node yaml.Node) error {
	if manifest == nil || node.Kind == 0 {
		return nil
	}

	switch node.Kind {
	case yaml.SequenceNode:
		var tools []string
		if err := node.Decode(&tools); err != nil {
			return fmt.Errorf("decode capabilities.tools: %w", err)
		}
		if len(manifest.Capabilities.AllowedTools) == 0 {
			manifest.Capabilities.AllowedTools = append([]string(nil), tools...)
		}
		manifest.Capabilities.Tools = append([]string(nil), tools...)
	case yaml.MappingNode:
		var toolConfig model.RoleToolConfig
		if err := node.Decode(&toolConfig); err != nil {
			return fmt.Errorf("decode capabilities.tools: %w", err)
		}
		manifest.Capabilities.ToolConfig = toolConfig
		if len(manifest.Capabilities.AllowedTools) == 0 {
			manifest.Capabilities.AllowedTools = append([]string(nil), toolConfig.BuiltIn...)
		}
		manifest.Capabilities.Tools = append([]string(nil), manifest.Capabilities.AllowedTools...)
	default:
		return fmt.Errorf("unsupported capabilities.tools shape")
	}

	return nil
}

func finalizeRoleManifest(manifest *Manifest) error {
	if manifest.Metadata.ID == "" {
		manifest.Metadata.ID = manifest.Metadata.Name
	}
	if manifest.Metadata.Name == "" {
		manifest.Metadata.Name = manifest.Metadata.ID
	}
	if manifest.Metadata.ID == "" {
		return fmt.Errorf("role metadata.id is required")
	}
	if manifest.Metadata.Name == "" {
		return fmt.Errorf("role metadata.name is required")
	}

	if manifest.Identity.Role == "" {
		manifest.Identity.Role = firstNonEmpty(manifest.Identity.Persona, manifest.Metadata.Name)
	}
	if manifest.Identity.Goal == "" && len(manifest.Identity.Goals) > 0 {
		manifest.Identity.Goal = strings.TrimSpace(manifest.Identity.Goals[0])
	}
	if manifest.Identity.Backstory == "" && manifest.Identity.Persona != "" {
		manifest.Identity.Backstory = manifest.Identity.Persona
	}

	manifest.SystemPrompt = firstNonEmpty(
		manifest.SystemPrompt,
		manifest.Identity.SystemPrompt,
		manifest.Knowledge.SystemPrompt,
		synthesizeSystemPrompt(manifest),
	)
	if manifest.Capabilities.MaxTurns <= 0 {
		manifest.Capabilities.MaxTurns = defaultMaxTurns
	}
	if manifest.Security.MaxBudgetUsd <= 0 && manifest.Capabilities.MaxBudgetUsd > 0 {
		manifest.Security.MaxBudgetUsd = manifest.Capabilities.MaxBudgetUsd
	}
	if manifest.Security.PermissionMode == "" {
		manifest.Security.PermissionMode = "default"
	}
	if len(manifest.Capabilities.Tools) == 0 {
		manifest.Capabilities.Tools = append([]string(nil), manifest.Capabilities.AllowedTools...)
	}
	skills, err := normalizeSkillReferences(manifest.Capabilities.Skills)
	if err != nil {
		return err
	}
	manifest.Capabilities.Skills = skills
	manifest.Knowledge.Shared = normalizeKnowledgeSources(manifest.Knowledge.Shared)
	manifest.Knowledge.Private = normalizeKnowledgeSources(manifest.Knowledge.Private)
	manifest.Security.OutputFilters = normalizeStringSlice(manifest.Security.OutputFilters)
	manifest.Security.Permissions.FileAccess.AllowedPaths = normalizeStringSlice(manifest.Security.Permissions.FileAccess.AllowedPaths)
	manifest.Security.Permissions.FileAccess.DeniedPaths = normalizeStringSlice(manifest.Security.Permissions.FileAccess.DeniedPaths)
	manifest.Security.Permissions.Network.AllowedDomains = normalizeStringSlice(manifest.Security.Permissions.Network.AllowedDomains)
	manifest.Security.Permissions.CodeExecution.AllowedLanguages = normalizeStringSlice(manifest.Security.Permissions.CodeExecution.AllowedLanguages)
	manifest.Collaboration.CanDelegateTo = normalizeStringSlice(manifest.Collaboration.CanDelegateTo)
	manifest.Collaboration.AcceptsDelegationFrom = normalizeStringSlice(manifest.Collaboration.AcceptsDelegationFrom)
	manifest.Triggers = normalizeTriggers(manifest.Triggers)

	return nil
}

func synthesizeSystemPrompt(manifest *Manifest) string {
	if manifest == nil {
		return ""
	}

	parts := make([]string, 0, 6)
	if roleName := strings.TrimSpace(firstNonEmpty(manifest.Identity.Role, manifest.Metadata.Name)); roleName != "" {
		parts = append(parts, fmt.Sprintf("You are %s.", roleName))
	}
	if goal := strings.TrimSpace(manifest.Identity.Goal); goal != "" {
		parts = append(parts, fmt.Sprintf("Your goal is to %s.", strings.TrimSuffix(goal, ".")))
	}
	if backstory := strings.TrimSpace(manifest.Identity.Backstory); backstory != "" {
		parts = append(parts, backstory)
	}
	if len(manifest.Identity.Constraints) > 0 {
		parts = append(parts, "Constraints:")
		for _, constraint := range manifest.Identity.Constraints {
			if trimmed := strings.TrimSpace(constraint); trimmed != "" {
				parts = append(parts, "- "+trimmed)
			}
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func normalizeSkillReferences(input []model.RoleSkillReference) ([]model.RoleSkillReference, error) {
	if len(input) == 0 {
		return nil, nil
	}

	normalized := make([]model.RoleSkillReference, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			return nil, fmt.Errorf("role capability skill path cannot be blank")
		}
		if _, ok := seen[path]; ok {
			return nil, fmt.Errorf("duplicate role capability skill path %q", path)
		}
		seen[path] = struct{}{}
		normalized = append(normalized, model.RoleSkillReference{
			Path:     path,
			AutoLoad: item.AutoLoad,
		})
	}

	return normalized, nil
}

func normalizeKnowledgeSources(input []model.RoleKnowledgeSource) []model.RoleKnowledgeSource {
	if len(input) == 0 {
		return nil
	}

	normalized := make([]model.RoleKnowledgeSource, 0, len(input))
	for _, item := range input {
		source := model.RoleKnowledgeSource{
			ID:          strings.TrimSpace(item.ID),
			Type:        strings.TrimSpace(item.Type),
			Access:      strings.TrimSpace(item.Access),
			Description: strings.TrimSpace(item.Description),
			Sources:     normalizeStringSlice(item.Sources),
		}
		if source.ID == "" && source.Type == "" && source.Access == "" && source.Description == "" && len(source.Sources) == 0 {
			continue
		}
		normalized = append(normalized, source)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeTriggers(input []model.RoleTrigger) []model.RoleTrigger {
	if len(input) == 0 {
		return nil
	}

	normalized := make([]model.RoleTrigger, 0, len(input))
	for _, item := range input {
		trigger := model.RoleTrigger{
			Event:            strings.TrimSpace(item.Event),
			Action:           strings.TrimSpace(item.Action),
			Condition:        strings.TrimSpace(item.Condition),
			AutoExecute:      item.AutoExecute,
			RequiresApproval: item.RequiresApproval,
		}
		if trigger.Event == "" && trigger.Action == "" && trigger.Condition == "" && !trigger.AutoExecute && !trigger.RequiresApproval {
			continue
		}
		normalized = append(normalized, trigger)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeStringSlice(input []string) []string {
	if len(input) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
