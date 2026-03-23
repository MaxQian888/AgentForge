package model

// RoleManifest represents a parsed role YAML file.
type RoleManifest struct {
	Metadata     RoleMetadata     `yaml:"metadata" json:"metadata"`
	Identity     RoleIdentity     `yaml:"identity" json:"identity"`
	Capabilities RoleCapabilities `yaml:"capabilities" json:"capabilities"`
	Knowledge    RoleKnowledge    `yaml:"knowledge" json:"knowledge"`
	Security     RoleSecurity     `yaml:"security" json:"security"`
}

type RoleMetadata struct {
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	Description string   `yaml:"description" json:"description"`
	Author      string   `yaml:"author" json:"author"`
	Tags        []string `yaml:"tags" json:"tags"`
}

type RoleIdentity struct {
	SystemPrompt string   `yaml:"system_prompt" json:"systemPrompt"`
	Persona      string   `yaml:"persona" json:"persona"`
	Goals        []string `yaml:"goals" json:"goals"`
	Constraints  []string `yaml:"constraints" json:"constraints"`
}

type RoleCapabilities struct {
	Tools          []string          `yaml:"tools" json:"tools"`
	Languages      []string          `yaml:"languages" json:"languages"`
	Frameworks     []string          `yaml:"frameworks" json:"frameworks"`
	MaxConcurrency int               `yaml:"max_concurrency" json:"maxConcurrency"`
	CustomSettings map[string]string `yaml:"custom_settings" json:"customSettings"`
}

type RoleKnowledge struct {
	Repositories []string `yaml:"repositories" json:"repositories"`
	Documents    []string `yaml:"documents" json:"documents"`
	Patterns     []string `yaml:"patterns" json:"patterns"`
}

type RoleSecurity struct {
	AllowedPaths  []string `yaml:"allowed_paths" json:"allowedPaths"`
	DeniedPaths   []string `yaml:"denied_paths" json:"deniedPaths"`
	MaxBudgetUsd  float64  `yaml:"max_budget_usd" json:"maxBudgetUsd"`
	RequireReview bool     `yaml:"require_review" json:"requireReview"`
}
