package model

// RoleManifest represents a parsed role YAML file.
type RoleManifest struct {
	APIVersion   string            `yaml:"apiVersion" json:"apiVersion"`
	Kind         string            `yaml:"kind" json:"kind"`
	Metadata     RoleMetadata      `yaml:"metadata" json:"metadata"`
	Identity     RoleIdentity      `yaml:"identity" json:"identity"`
	SystemPrompt string            `yaml:"system_prompt,omitempty" json:"systemPrompt,omitempty"`
	Capabilities RoleCapabilities  `yaml:"capabilities" json:"capabilities"`
	Knowledge    RoleKnowledge     `yaml:"knowledge" json:"knowledge"`
	Security     RoleSecurity      `yaml:"security" json:"security"`
	Extends      string            `yaml:"extends,omitempty" json:"extends,omitempty"`
	Overrides    map[string]any    `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	Collaboration map[string]any   `yaml:"collaboration,omitempty" json:"collaboration,omitempty"`
	Triggers      []map[string]any `yaml:"triggers,omitempty" json:"triggers,omitempty"`
}

type RoleMetadata struct {
	ID          string   `yaml:"id,omitempty" json:"id,omitempty"`
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	Description string   `yaml:"description" json:"description"`
	Author      string   `yaml:"author" json:"author"`
	Tags        []string `yaml:"tags" json:"tags"`
	Icon        string   `yaml:"icon,omitempty" json:"icon,omitempty"`
}

type RoleIdentity struct {
	Role         string               `yaml:"role,omitempty" json:"role,omitempty"`
	Goal         string               `yaml:"goal,omitempty" json:"goal,omitempty"`
	Backstory    string               `yaml:"backstory,omitempty" json:"backstory,omitempty"`
	SystemPrompt  string            `yaml:"system_prompt" json:"systemPrompt"`
	Persona       string            `yaml:"persona" json:"persona"`
	Goals         []string          `yaml:"goals" json:"goals"`
	Constraints   []string          `yaml:"constraints" json:"constraints"`
	Personality   string            `yaml:"personality,omitempty" json:"personality,omitempty"`
	Language      string            `yaml:"language,omitempty" json:"language,omitempty"`
	ResponseStyle RoleResponseStyle `yaml:"response_style,omitempty" json:"responseStyle,omitempty"`
}

type RoleCapabilities struct {
	Packages       []string          `yaml:"packages,omitempty" json:"packages,omitempty"`
	AllowedTools   []string          `yaml:"allowed_tools,omitempty" json:"allowedTools,omitempty"`
	Tools          []string          `yaml:"-" json:"tools,omitempty"`
	ToolConfig     RoleToolConfig    `yaml:"tools,omitempty" json:"toolConfig,omitempty"`
	Languages      []string          `yaml:"languages" json:"languages"`
	Frameworks     []string          `yaml:"frameworks" json:"frameworks"`
	MaxConcurrency int               `yaml:"max_concurrency" json:"maxConcurrency"`
	MaxTurns       int               `yaml:"max_turns,omitempty" json:"maxTurns,omitempty"`
	MaxBudgetUsd   float64           `yaml:"max_budget_usd,omitempty" json:"maxBudgetUsd,omitempty"`
	CustomSettings map[string]string `yaml:"custom_settings" json:"customSettings"`
}

type RoleKnowledge struct {
	Repositories []string `yaml:"repositories" json:"repositories"`
	Documents    []string `yaml:"documents" json:"documents"`
	Patterns     []string `yaml:"patterns" json:"patterns"`
	SystemPrompt string   `yaml:"system_prompt,omitempty" json:"systemPrompt,omitempty"`
}

type RoleSecurity struct {
	PermissionMode string   `yaml:"permission_mode,omitempty" json:"permissionMode,omitempty"`
	AllowedPaths  []string `yaml:"allowed_paths" json:"allowedPaths"`
	DeniedPaths   []string `yaml:"denied_paths" json:"deniedPaths"`
	MaxBudgetUsd  float64  `yaml:"max_budget_usd" json:"maxBudgetUsd"`
	RequireReview bool     `yaml:"require_review" json:"requireReview"`
}

type RoleResponseStyle struct {
	Tone             string `yaml:"tone,omitempty" json:"tone,omitempty"`
	Verbosity        string `yaml:"verbosity,omitempty" json:"verbosity,omitempty"`
	FormatPreference string `yaml:"format_preference,omitempty" json:"formatPreference,omitempty"`
}

type RoleToolConfig struct {
	BuiltIn   []string        `yaml:"built_in,omitempty" json:"builtIn,omitempty"`
	External  []string        `yaml:"external,omitempty" json:"external,omitempty"`
	MCPServers []RoleMCPServer `yaml:"mcp_servers,omitempty" json:"mcpServers,omitempty"`
}

type RoleMCPServer struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	URL  string `yaml:"url,omitempty" json:"url,omitempty"`
}

type RoleExecutionProfile struct {
	RoleID         string   `json:"role_id"`
	Name           string   `json:"name"`
	Role           string   `json:"role"`
	Goal           string   `json:"goal"`
	Backstory      string   `json:"backstory"`
	SystemPrompt   string   `json:"system_prompt"`
	AllowedTools   []string `json:"allowed_tools"`
	MaxBudgetUsd   float64  `json:"max_budget_usd"`
	MaxTurns       int      `json:"max_turns"`
	PermissionMode string   `json:"permission_mode"`
}
