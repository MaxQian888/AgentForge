package model

// RoleManifest represents a parsed role YAML file.
type RoleManifest struct {
	APIVersion    string            `yaml:"apiVersion" json:"apiVersion"`
	Kind          string            `yaml:"kind" json:"kind"`
	Metadata      RoleMetadata      `yaml:"metadata" json:"metadata"`
	Identity      RoleIdentity      `yaml:"identity" json:"identity"`
	SystemPrompt  string            `yaml:"system_prompt,omitempty" json:"systemPrompt,omitempty"`
	Capabilities  RoleCapabilities  `yaml:"capabilities" json:"capabilities"`
	Knowledge     RoleKnowledge     `yaml:"knowledge" json:"knowledge"`
	Security      RoleSecurity      `yaml:"security" json:"security"`
	Extends       string            `yaml:"extends,omitempty" json:"extends,omitempty"`
	Overrides     map[string]any    `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	Collaboration RoleCollaboration `yaml:"collaboration,omitempty" json:"collaboration,omitempty"`
	Triggers      []RoleTrigger     `yaml:"triggers,omitempty" json:"triggers,omitempty"`
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
	Role          string            `yaml:"role,omitempty" json:"role,omitempty"`
	Goal          string            `yaml:"goal,omitempty" json:"goal,omitempty"`
	Backstory     string            `yaml:"backstory,omitempty" json:"backstory,omitempty"`
	SystemPrompt  string            `yaml:"system_prompt" json:"systemPrompt"`
	Persona       string            `yaml:"persona" json:"persona"`
	Goals         []string          `yaml:"goals" json:"goals"`
	Constraints   []string          `yaml:"constraints" json:"constraints"`
	Personality   string            `yaml:"personality,omitempty" json:"personality,omitempty"`
	Language      string            `yaml:"language,omitempty" json:"language,omitempty"`
	ResponseStyle RoleResponseStyle `yaml:"response_style,omitempty" json:"responseStyle,omitempty"`
}

type RoleCapabilities struct {
	Packages       []string             `yaml:"packages,omitempty" json:"packages,omitempty"`
	AllowedTools   []string             `yaml:"allowed_tools,omitempty" json:"allowedTools,omitempty"`
	Tools          []string             `yaml:"-" json:"tools,omitempty"`
	ToolConfig     RoleToolConfig       `yaml:"tools,omitempty" json:"toolConfig,omitempty"`
	Skills         []RoleSkillReference `yaml:"skills,omitempty" json:"skills,omitempty"`
	Languages      []string             `yaml:"languages" json:"languages"`
	Frameworks     []string             `yaml:"frameworks" json:"frameworks"`
	MaxConcurrency int                  `yaml:"max_concurrency" json:"maxConcurrency"`
	MaxTurns       int                  `yaml:"max_turns,omitempty" json:"maxTurns,omitempty"`
	MaxBudgetUsd   float64              `yaml:"max_budget_usd,omitempty" json:"maxBudgetUsd,omitempty"`
	CustomSettings map[string]string    `yaml:"custom_settings" json:"customSettings"`
}

type RoleSkillReference struct {
	Path     string `yaml:"path" json:"path"`
	AutoLoad bool   `yaml:"auto_load,omitempty" json:"autoLoad"`
}

type RoleKnowledge struct {
	Repositories []string              `yaml:"repositories" json:"repositories"`
	Documents    []string              `yaml:"documents" json:"documents"`
	Patterns     []string              `yaml:"patterns" json:"patterns"`
	SystemPrompt string                `yaml:"system_prompt,omitempty" json:"systemPrompt,omitempty"`
	Shared       []RoleKnowledgeSource `yaml:"shared,omitempty" json:"shared,omitempty"`
	Private      []RoleKnowledgeSource `yaml:"private,omitempty" json:"private,omitempty"`
	Memory       RoleMemoryConfig      `yaml:"memory,omitempty" json:"memory,omitempty"`
}

type RoleSecurity struct {
	PermissionMode string             `yaml:"permission_mode,omitempty" json:"permissionMode,omitempty"`
	AllowedPaths   []string           `yaml:"allowed_paths" json:"allowedPaths"`
	DeniedPaths    []string           `yaml:"denied_paths" json:"deniedPaths"`
	MaxBudgetUsd   float64            `yaml:"max_budget_usd" json:"maxBudgetUsd"`
	RequireReview  bool               `yaml:"require_review" json:"requireReview"`
	Profile        string             `yaml:"profile,omitempty" json:"profile,omitempty"`
	Permissions    RolePermissions    `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	OutputFilters  []string           `yaml:"output_filters,omitempty" json:"outputFilters,omitempty"`
	ResourceLimits RoleResourceLimits `yaml:"resource_limits,omitempty" json:"resourceLimits,omitempty"`
}

type RoleResponseStyle struct {
	Tone             string `yaml:"tone,omitempty" json:"tone,omitempty"`
	Verbosity        string `yaml:"verbosity,omitempty" json:"verbosity,omitempty"`
	FormatPreference string `yaml:"format_preference,omitempty" json:"formatPreference,omitempty"`
}

type RoleToolConfig struct {
	BuiltIn    []string        `yaml:"built_in,omitempty" json:"builtIn,omitempty"`
	External   []string        `yaml:"external,omitempty" json:"external,omitempty"`
	MCPServers []RoleMCPServer `yaml:"mcp_servers,omitempty" json:"mcpServers,omitempty"`
}

type RoleMCPServer struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	URL  string `yaml:"url,omitempty" json:"url,omitempty"`
}

type RoleKnowledgeSource struct {
	ID          string   `yaml:"id,omitempty" json:"id,omitempty"`
	Type        string   `yaml:"type,omitempty" json:"type,omitempty"`
	Access      string   `yaml:"access,omitempty" json:"access,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Sources     []string `yaml:"sources,omitempty" json:"sources,omitempty"`
}

type RoleMemoryConfig struct {
	ShortTerm  RoleShortTermMemory  `yaml:"short_term,omitempty" json:"shortTerm,omitempty"`
	Episodic   RoleEpisodicMemory   `yaml:"episodic,omitempty" json:"episodic,omitempty"`
	Semantic   RoleSemanticMemory   `yaml:"semantic,omitempty" json:"semantic,omitempty"`
	Procedural RoleProceduralMemory `yaml:"procedural,omitempty" json:"procedural,omitempty"`
}

type RoleShortTermMemory struct {
	MaxTokens int `yaml:"max_tokens,omitempty" json:"maxTokens,omitempty"`
}

type RoleEpisodicMemory struct {
	Enabled       bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	RetentionDays int  `yaml:"retention_days,omitempty" json:"retentionDays,omitempty"`
}

type RoleSemanticMemory struct {
	Enabled     bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	AutoExtract bool `yaml:"auto_extract,omitempty" json:"autoExtract,omitempty"`
}

type RoleProceduralMemory struct {
	Enabled           bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	LearnFromFeedback bool `yaml:"learn_from_feedback,omitempty" json:"learnFromFeedback,omitempty"`
}

type RolePermissions struct {
	FileAccess    RoleFileAccessPermission    `yaml:"file_access,omitempty" json:"fileAccess,omitempty"`
	Network       RoleNetworkPermission       `yaml:"network,omitempty" json:"network,omitempty"`
	CodeExecution RoleCodeExecutionPermission `yaml:"code_execution,omitempty" json:"codeExecution,omitempty"`
}

type RoleFileAccessPermission struct {
	AllowedPaths []string `yaml:"allowed_paths,omitempty" json:"allowedPaths,omitempty"`
	DeniedPaths  []string `yaml:"denied_paths,omitempty" json:"deniedPaths,omitempty"`
}

type RoleNetworkPermission struct {
	AllowedDomains []string `yaml:"allowed_domains,omitempty" json:"allowedDomains,omitempty"`
}

type RoleCodeExecutionPermission struct {
	Sandbox          bool     `yaml:"sandbox,omitempty" json:"sandbox,omitempty"`
	AllowedLanguages []string `yaml:"allowed_languages,omitempty" json:"allowedLanguages,omitempty"`
}

type RoleResourceLimits struct {
	TokenBudget   RoleTokenBudgetLimit   `yaml:"token_budget,omitempty" json:"tokenBudget,omitempty"`
	APICalls      RoleAPICallsLimit      `yaml:"api_calls,omitempty" json:"apiCalls,omitempty"`
	ExecutionTime RoleExecutionTimeLimit `yaml:"execution_time,omitempty" json:"executionTime,omitempty"`
	CostLimit     RoleCostLimit          `yaml:"cost_limit,omitempty" json:"costLimit,omitempty"`
}

type RoleTokenBudgetLimit struct {
	PerTask  int `yaml:"per_task,omitempty" json:"perTask,omitempty"`
	PerDay   int `yaml:"per_day,omitempty" json:"perDay,omitempty"`
	PerMonth int `yaml:"per_month,omitempty" json:"perMonth,omitempty"`
}

type RoleAPICallsLimit struct {
	PerMinute int `yaml:"per_minute,omitempty" json:"perMinute,omitempty"`
	PerHour   int `yaml:"per_hour,omitempty" json:"perHour,omitempty"`
}

type RoleExecutionTimeLimit struct {
	PerTask string `yaml:"per_task,omitempty" json:"perTask,omitempty"`
	PerDay  string `yaml:"per_day,omitempty" json:"perDay,omitempty"`
}

type RoleCostLimit struct {
	PerTask        string  `yaml:"per_task,omitempty" json:"perTask,omitempty"`
	PerDay         string  `yaml:"per_day,omitempty" json:"perDay,omitempty"`
	AlertThreshold float64 `yaml:"alert_threshold,omitempty" json:"alertThreshold,omitempty"`
}

type RoleCollaboration struct {
	CanDelegateTo         []string               `yaml:"can_delegate_to,omitempty" json:"canDelegateTo,omitempty"`
	AcceptsDelegationFrom []string               `yaml:"accepts_delegation_from,omitempty" json:"acceptsDelegationFrom,omitempty"`
	Communication         RoleCommunicationPrefs `yaml:"communication,omitempty" json:"communication,omitempty"`
}

type RoleCommunicationPrefs struct {
	PreferredChannel string `yaml:"preferred_channel,omitempty" json:"preferredChannel,omitempty"`
	ReportFormat     string `yaml:"report_format,omitempty" json:"reportFormat,omitempty"`
	EscalationPolicy string `yaml:"escalation_policy,omitempty" json:"escalationPolicy,omitempty"`
}

type RoleTrigger struct {
	Event            string `yaml:"event,omitempty" json:"event,omitempty"`
	Action           string `yaml:"action,omitempty" json:"action,omitempty"`
	Condition        string `yaml:"condition,omitempty" json:"condition,omitempty"`
	AutoExecute      bool   `yaml:"auto_execute,omitempty" json:"autoExecute,omitempty"`
	RequiresApproval bool   `yaml:"requires_approval,omitempty" json:"requiresApproval,omitempty"`
}

type RoleExecutionProfile struct {
	RoleID           string   `json:"role_id"`
	Name             string   `json:"name"`
	Role             string   `json:"role"`
	Goal             string   `json:"goal"`
	Backstory        string   `json:"backstory"`
	SystemPrompt     string   `json:"system_prompt"`
	AllowedTools     []string `json:"allowed_tools"`
	Tools            []string `json:"tools,omitempty"`
	KnowledgeContext string   `json:"knowledge_context,omitempty"`
	OutputFilters    []string `json:"output_filters,omitempty"`
	MaxBudgetUsd     float64  `json:"max_budget_usd"`
	MaxTurns         int      `json:"max_turns"`
	PermissionMode   string   `json:"permission_mode"`
	LoadedSkills     []RoleExecutionSkill           `json:"loaded_skills,omitempty"`
	AvailableSkills  []RoleExecutionSkill           `json:"available_skills,omitempty"`
	SkillDiagnostics []RoleExecutionSkillDiagnostic `json:"skill_diagnostics,omitempty"`
}

type RoleExecutionSkill struct {
	Path         string   `json:"path"`
	Label        string   `json:"label"`
	Description  string   `json:"description,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
	Source       string   `json:"source,omitempty"`
	SourceRoot   string   `json:"source_root,omitempty"`
	Origin       string   `json:"origin,omitempty"`
	Requires     []string `json:"requires,omitempty"`
	Tools        []string `json:"tools,omitempty"`
}

type RoleExecutionSkillDiagnostic struct {
	Code     string `json:"code"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
	AutoLoad bool   `json:"auto_load,omitempty"`
}
