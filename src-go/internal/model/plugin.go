package model

import "time"

type PluginKind string

const (
	PluginKindRole        PluginKind = "RolePlugin"
	PluginKindTool        PluginKind = "ToolPlugin"
	PluginKindWorkflow    PluginKind = "WorkflowPlugin"
	PluginKindIntegration PluginKind = "IntegrationPlugin"
	PluginKindReview      PluginKind = "ReviewPlugin"
)

type PluginRuntime string

const (
	PluginRuntimeDeclarative      PluginRuntime = "declarative"
	PluginRuntimeMCP              PluginRuntime = "mcp"
	PluginRuntimeGoPlugin         PluginRuntime = "go-plugin"
	PluginRuntimeWASM             PluginRuntime = "wasm"
	// PluginRuntimeFirstpartyInproc marks integration plugins whose code
	// ships inside the Go monolith and is wired via an in-proc
	// registration hook (e.g. src-go/internal/server/qianchuan_plugin.go).
	// They declare their manifest on disk so the control plane surfaces
	// them consistently, but need no external binary/module.
	PluginRuntimeFirstpartyInproc PluginRuntime = "firstparty-inproc"
)

type PluginSourceType string

const (
	PluginSourceBuiltin     PluginSourceType = "builtin"
	PluginSourceLocal       PluginSourceType = "local"
	PluginSourceGit         PluginSourceType = "git"
	PluginSourceNPM         PluginSourceType = "npm"
	PluginSourceCatalog     PluginSourceType = "catalog"
	PluginSourceMarketplace PluginSourceType = "marketplace"
)

type PluginTrustState string

const (
	PluginTrustUnknown   PluginTrustState = "unknown"
	PluginTrustVerified  PluginTrustState = "verified"
	PluginTrustUntrusted PluginTrustState = "untrusted"
)

type PluginApprovalState string

const (
	PluginApprovalNotRequired PluginApprovalState = "not-required"
	PluginApprovalPending     PluginApprovalState = "pending"
	PluginApprovalApproved    PluginApprovalState = "approved"
	PluginApprovalRejected    PluginApprovalState = "rejected"
)

type PluginLifecycleOperation string

const (
	PluginOpInstall    PluginLifecycleOperation = "install"
	PluginOpEnable     PluginLifecycleOperation = "enable"
	PluginOpActivate   PluginLifecycleOperation = "activate"
	PluginOpDeactivate PluginLifecycleOperation = "deactivate"
	PluginOpDisable    PluginLifecycleOperation = "disable"
	PluginOpUninstall  PluginLifecycleOperation = "uninstall"
	PluginOpUpdate     PluginLifecycleOperation = "update"
)

type PluginLifecycleState string

const (
	PluginStateInstalled  PluginLifecycleState = "installed"
	PluginStateEnabled    PluginLifecycleState = "enabled"
	PluginStateActivating PluginLifecycleState = "activating"
	PluginStateActive     PluginLifecycleState = "active"
	PluginStateDegraded   PluginLifecycleState = "degraded"
	PluginStateDisabled   PluginLifecycleState = "disabled"
)

type PluginRuntimeHost string

const (
	PluginHostGoOrchestrator PluginRuntimeHost = "go-orchestrator"
	PluginHostTSBridge       PluginRuntimeHost = "ts-bridge"
)

type PluginManifest struct {
	APIVersion  string            `yaml:"apiVersion" json:"apiVersion"`
	Kind        PluginKind        `yaml:"kind" json:"kind"`
	Metadata    PluginMetadata    `yaml:"metadata" json:"metadata"`
	Spec        PluginSpec        `yaml:"spec" json:"spec"`
	Permissions PluginPermissions `yaml:"permissions" json:"permissions"`
	Source      PluginSource      `yaml:"source" json:"source"`
}

type PluginMetadata struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

type PluginSpec struct {
	Runtime      PluginRuntime       `yaml:"runtime" json:"runtime"`
	Transport    string              `yaml:"transport,omitempty" json:"transport,omitempty"`
	Command      string              `yaml:"command,omitempty" json:"command,omitempty"`
	Args         []string            `yaml:"args,omitempty" json:"args,omitempty"`
	URL          string              `yaml:"url,omitempty" json:"url,omitempty"`
	Binary       string              `yaml:"binary,omitempty" json:"binary,omitempty"`
	Module       string              `yaml:"module,omitempty" json:"module,omitempty"`
	ABIVersion   string              `yaml:"abiVersion,omitempty" json:"abiVersion,omitempty"`
	Capabilities []string            `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Config       map[string]any      `yaml:"config,omitempty" json:"config,omitempty"`
	ConfigSchema *PluginConfigSchema `yaml:"configSchema,omitempty" json:"configSchema,omitempty"`
	Workflow     *WorkflowPluginSpec `yaml:"workflow,omitempty" json:"workflow,omitempty"`
	Review       *ReviewPluginSpec   `yaml:"review,omitempty" json:"review,omitempty"`
	Extra        map[string]any      `yaml:",inline" json:"extra,omitempty"`
}

// PluginConfigSchema is a minimal JSON-Schema subset that plugin
// manifests can declare to validate their spec.config at UpdateConfig
// time. Supported fields: type (string/integer/number/boolean/object/array),
// required (required keys when type=object), properties (nested schemas
// keyed by field name), enum (allowed literal values),
// additionalProperties (reject unknown keys when false). Unsupported
// JSON Schema features are ignored — keep schemas small and explicit.
type PluginConfigSchema struct {
	Type                 string                         `yaml:"type,omitempty" json:"type,omitempty"`
	Required             []string                       `yaml:"required,omitempty" json:"required,omitempty"`
	Properties           map[string]*PluginConfigSchema `yaml:"properties,omitempty" json:"properties,omitempty"`
	Items                *PluginConfigSchema            `yaml:"items,omitempty" json:"items,omitempty"`
	Enum                 []any                          `yaml:"enum,omitempty" json:"enum,omitempty"`
	AdditionalProperties *bool                          `yaml:"additionalProperties,omitempty" json:"additionalProperties,omitempty"`
}

type WorkflowProcessMode string

const (
	WorkflowProcessSequential   WorkflowProcessMode = "sequential"
	WorkflowProcessHierarchical WorkflowProcessMode = "hierarchical"
	WorkflowProcessEventDriven  WorkflowProcessMode = "event-driven"
	WorkflowProcessWave         WorkflowProcessMode = "wave"
)

type WorkflowActionType string

const (
	WorkflowActionAgent    WorkflowActionType = "agent"
	WorkflowActionReview   WorkflowActionType = "review"
	WorkflowActionTask     WorkflowActionType = "task"
	WorkflowActionWorkflow WorkflowActionType = "workflow"
	WorkflowActionApproval WorkflowActionType = "approval"
)

type WorkflowPluginSpec struct {
	Process  WorkflowProcessMode      `yaml:"process" json:"process"`
	Roles    []WorkflowRoleBinding    `yaml:"roles,omitempty" json:"roles,omitempty"`
	Steps    []WorkflowStepDefinition `yaml:"steps,omitempty" json:"steps,omitempty"`
	Triggers []PluginWorkflowTrigger  `yaml:"triggers,omitempty" json:"triggers,omitempty"`
	Limits   *WorkflowExecutionLimits `yaml:"limits,omitempty" json:"limits,omitempty"`

	// Hierarchical-mode fields. ManagerRole and WorkerRoles together drive
	// the manager → workers → aggregate flow; MaxParallelWorkers caps
	// concurrent worker dispatch (0 = no cap, all workers run in parallel).
	// WorkerFailurePolicy is "best_effort" (default) or "fail_fast".
	ManagerRole         string   `yaml:"managerRole,omitempty" json:"managerRole,omitempty"`
	WorkerRoles         []string `yaml:"workerRoles,omitempty" json:"workerRoles,omitempty"`
	MaxParallelWorkers  int      `yaml:"maxParallelWorkers,omitempty" json:"maxParallelWorkers,omitempty"`
	WorkerFailurePolicy string   `yaml:"workerFailurePolicy,omitempty" json:"workerFailurePolicy,omitempty"`
	Aggregation         string   `yaml:"aggregation,omitempty" json:"aggregation,omitempty"`
}

type WorkflowRoleBinding struct {
	ID string `yaml:"id" json:"id"`
}

type WorkflowStepDefinition struct {
	ID       string             `yaml:"id" json:"id"`
	Role     string             `yaml:"role" json:"role"`
	Action   WorkflowActionType `yaml:"action" json:"action"`
	Next     []string           `yaml:"next,omitempty" json:"next,omitempty"`
	Config   map[string]any     `yaml:"config,omitempty" json:"config,omitempty"`
	Metadata map[string]any     `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type PluginWorkflowTrigger struct {
	Event        string `yaml:"event,omitempty" json:"event,omitempty"`
	Profile      string `yaml:"profile,omitempty" json:"profile,omitempty"`
	RequiresTask bool   `yaml:"requiresTask,omitempty" json:"requiresTask,omitempty"`

	// Event-driven-mode fields. Filter is a key→value match against the event
	// payload (all keys must match for the trigger to fire). Role + Action
	// describe the workflow step the executor dispatches when matched.
	// MaxConcurrent caps in-flight dispatches per trigger (0 = 1).
	Filter        map[string]any `yaml:"filter,omitempty" json:"filter,omitempty"`
	Role          string         `yaml:"role,omitempty" json:"role,omitempty"`
	Action        string         `yaml:"action,omitempty" json:"action,omitempty"`
	MaxConcurrent int            `yaml:"maxConcurrent,omitempty" json:"maxConcurrent,omitempty"`
}

type WorkflowExecutionLimits struct {
	MaxRetries int `yaml:"maxRetries,omitempty" json:"maxRetries,omitempty"`
}

type ReviewPluginSpec struct {
	Entrypoint string              `yaml:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	Triggers   ReviewPluginTrigger `yaml:"triggers" json:"triggers"`
	Output     ReviewPluginOutput  `yaml:"output" json:"output"`
}

type ReviewPluginTrigger struct {
	Events       []string `yaml:"events,omitempty" json:"events,omitempty"`
	FilePatterns []string `yaml:"filePatterns,omitempty" json:"filePatterns,omitempty"`
}

type ReviewPluginOutput struct {
	Format string `yaml:"format" json:"format"`
}

type PluginRuntimeMetadata struct {
	ABIVersion string                    `json:"abi_version,omitempty"`
	Compatible bool                      `json:"compatible"`
	MCP        *PluginMCPRuntimeMetadata `json:"mcp,omitempty"`
}

type MCPInteractionOperation string

const (
	MCPInteractionRefresh      MCPInteractionOperation = "refresh"
	MCPInteractionCallTool     MCPInteractionOperation = "call_tool"
	MCPInteractionReadResource MCPInteractionOperation = "read_resource"
	MCPInteractionGetPrompt    MCPInteractionOperation = "get_prompt"
)

type MCPInteractionStatus string

const (
	MCPInteractionSucceeded MCPInteractionStatus = "succeeded"
	MCPInteractionFailed    MCPInteractionStatus = "failed"
)

type MCPInteractionSummary struct {
	Operation    MCPInteractionOperation `json:"operation"`
	Status       MCPInteractionStatus    `json:"status"`
	At           *time.Time              `json:"at,omitempty"`
	Target       string                  `json:"target,omitempty"`
	Summary      string                  `json:"summary,omitempty"`
	ErrorCode    string                  `json:"error_code,omitempty"`
	ErrorMessage string                  `json:"error_message,omitempty"`
}

type PluginMCPRuntimeMetadata struct {
	Transport         string                 `json:"transport"`
	LastDiscoveryAt   *time.Time             `json:"last_discovery_at,omitempty"`
	ToolCount         int                    `json:"tool_count"`
	ResourceCount     int                    `json:"resource_count"`
	PromptCount       int                    `json:"prompt_count"`
	LatestInteraction *MCPInteractionSummary `json:"latest_interaction,omitempty"`
}

type MCPCapabilityTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type MCPCapabilityResource struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

type MCPCapabilityPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type PluginMCPCapabilitySnapshot struct {
	Transport         string                  `json:"transport"`
	LastDiscoveryAt   *time.Time              `json:"last_discovery_at,omitempty"`
	ToolCount         int                     `json:"tool_count"`
	ResourceCount     int                     `json:"resource_count"`
	PromptCount       int                     `json:"prompt_count"`
	Tools             []MCPCapabilityTool     `json:"tools,omitempty"`
	Resources         []MCPCapabilityResource `json:"resources,omitempty"`
	Prompts           []MCPCapabilityPrompt   `json:"prompts,omitempty"`
	LatestInteraction *MCPInteractionSummary  `json:"latest_interaction,omitempty"`
}

type PluginMCPRefreshResult struct {
	PluginID        string                      `json:"plugin_id"`
	LifecycleState  PluginLifecycleState        `json:"lifecycle_state,omitempty"`
	RuntimeHost     PluginRuntimeHost           `json:"runtime_host,omitempty"`
	RuntimeMetadata *PluginRuntimeMetadata      `json:"runtime_metadata,omitempty"`
	Snapshot        PluginMCPCapabilitySnapshot `json:"snapshot"`
}

type MCPContentBlock struct {
	Type     string `json:"type,omitempty"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty"`
}

type MCPToolCallResult struct {
	Content           []MCPContentBlock `json:"content,omitempty"`
	IsError           bool              `json:"isError"`
	StructuredContent map[string]any    `json:"structuredContent,omitempty"`
}

type PluginMCPToolCallResult struct {
	PluginID  string            `json:"plugin_id"`
	Operation string            `json:"operation"`
	Result    MCPToolCallResult `json:"result"`
}

type MCPResourceContent struct {
	URI      string `json:"uri,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

type MCPResourceReadResult struct {
	Contents []MCPResourceContent `json:"contents,omitempty"`
}

type PluginMCPResourceReadResult struct {
	PluginID  string                `json:"plugin_id"`
	Operation string                `json:"operation"`
	Result    MCPResourceReadResult `json:"result"`
}

type MCPPromptMessageContent struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type MCPPromptMessage struct {
	Role    string                  `json:"role,omitempty"`
	Content MCPPromptMessageContent `json:"content"`
}

type MCPPromptGetResult struct {
	Description string             `json:"description,omitempty"`
	Messages    []MCPPromptMessage `json:"messages,omitempty"`
}

type PluginMCPPromptResult struct {
	PluginID  string             `json:"plugin_id"`
	Operation string             `json:"operation"`
	Result    MCPPromptGetResult `json:"result"`
}

type PluginPermissions struct {
	Network    *PluginNetworkPermission    `yaml:"network,omitempty" json:"network,omitempty"`
	Filesystem *PluginFilesystemPermission `yaml:"filesystem,omitempty" json:"filesystem,omitempty"`
}

type PluginNetworkPermission struct {
	Required bool     `yaml:"required" json:"required"`
	Domains  []string `yaml:"domains,omitempty" json:"domains,omitempty"`
}

type PluginFilesystemPermission struct {
	Required     bool     `yaml:"required" json:"required"`
	AllowedPaths []string `yaml:"allowed_paths,omitempty" json:"allowed_paths,omitempty"`
}

type PluginSource struct {
	Type       PluginSourceType       `yaml:"type,omitempty" json:"type"`
	Path       string                 `yaml:"path,omitempty" json:"path,omitempty"`
	Repository string                 `yaml:"repository,omitempty" json:"repository,omitempty"`
	Ref        string                 `yaml:"ref,omitempty" json:"ref,omitempty"`
	Package    string                 `yaml:"package,omitempty" json:"package,omitempty"`
	Version    string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Registry   string                 `yaml:"registry,omitempty" json:"registry,omitempty"`
	Catalog    string                 `yaml:"catalog,omitempty" json:"catalog,omitempty"`
	Entry      string                 `yaml:"entry,omitempty" json:"entry,omitempty"`
	Digest     string                 `yaml:"digest,omitempty" json:"digest,omitempty"`
	Signature  string                 `yaml:"signature,omitempty" json:"signature,omitempty"`
	Trust      *PluginTrustMetadata   `yaml:"trust,omitempty" json:"trust,omitempty"`
	Release    *PluginReleaseMetadata `yaml:"release,omitempty" json:"release,omitempty"`
}

type PluginTrustMetadata struct {
	Status        PluginTrustState    `yaml:"status,omitempty" json:"status,omitempty"`
	ApprovalState PluginApprovalState `yaml:"approvalState,omitempty" json:"approvalState,omitempty"`
	Source        string              `yaml:"source,omitempty" json:"source,omitempty"`
	VerifiedAt    *time.Time          `yaml:"verifiedAt,omitempty" json:"verifiedAt,omitempty"`
	ApprovedBy    string              `yaml:"approvedBy,omitempty" json:"approvedBy,omitempty"`
	ApprovedAt    *time.Time          `yaml:"approvedAt,omitempty" json:"approvedAt,omitempty"`
	Reason        string              `yaml:"reason,omitempty" json:"reason,omitempty"`
}

type PluginReleaseMetadata struct {
	Version          string     `yaml:"version,omitempty" json:"version,omitempty"`
	Channel          string     `yaml:"channel,omitempty" json:"channel,omitempty"`
	Artifact         string     `yaml:"artifact,omitempty" json:"artifact,omitempty"`
	NotesURL         string     `yaml:"notesUrl,omitempty" json:"notesUrl,omitempty"`
	PublishedAt      *time.Time `yaml:"publishedAt,omitempty" json:"publishedAt,omitempty"`
	AvailableVersion string     `yaml:"availableVersion,omitempty" json:"availableVersion,omitempty"`
}

type PluginBuiltInMetadata struct {
	Official             bool     `json:"official,omitempty"`
	DocsRef              string   `json:"docsRef,omitempty"`
	VerificationProfile  string   `json:"verificationProfile,omitempty"`
	CoreFlows            []string `json:"coreFlows,omitempty"`
	StarterFamily        string   `json:"starterFamily,omitempty"`
	DependencyRefs       []string `json:"dependencyRefs,omitempty"`
	WorkspaceRefs        []string `json:"workspaceRefs,omitempty"`
	AvailabilityStatus   string   `json:"availabilityStatus,omitempty"`
	AvailabilityMessage  string   `json:"availabilityMessage,omitempty"`
	ReadinessStatus      string   `json:"readinessStatus,omitempty"`
	ReadinessMessage     string   `json:"readinessMessage,omitempty"`
	NextStep             string   `json:"nextStep,omitempty"`
	BlockingReasons      []string `json:"blockingReasons,omitempty"`
	MissingPrerequisites []string `json:"missingPrerequisites,omitempty"`
	MissingConfiguration []string `json:"missingConfiguration,omitempty"`
	Installable          bool     `json:"installable"`
	InstallBlockedReason string   `json:"installBlockedReason,omitempty"`
}

type PluginRoleDependency struct {
	RoleID     string   `json:"roleId"`
	RoleName   string   `json:"roleName,omitempty"`
	Status     string   `json:"status"`
	Blocking   bool     `json:"blocking"`
	Message    string   `json:"message,omitempty"`
	References []string `json:"references,omitempty"`
}

type PluginRoleConsumer struct {
	RoleID        string `json:"roleId"`
	RoleName      string `json:"roleName,omitempty"`
	ReferenceType string `json:"referenceType"`
	Status        string `json:"status"`
	Blocking      bool   `json:"blocking"`
	Message       string `json:"message,omitempty"`
}

type PluginRecord struct {
	PluginManifest
	LifecycleState     PluginLifecycleState    `json:"lifecycle_state"`
	RuntimeHost        PluginRuntimeHost       `json:"runtime_host,omitempty"`
	LastHealthAt       *time.Time              `json:"last_health_at,omitempty"`
	LastError          string                  `json:"last_error,omitempty"`
	RestartCount       int                     `json:"restart_count"`
	ResolvedSourcePath string                  `json:"resolved_source_path,omitempty"`
	RuntimeMetadata    *PluginRuntimeMetadata  `json:"runtime_metadata,omitempty"`
	CurrentInstance    *PluginInstanceSnapshot `json:"current_instance,omitempty"`
	BuiltIn            *PluginBuiltInMetadata  `json:"builtIn,omitempty"`
	RoleDependencies   []PluginRoleDependency  `json:"roleDependencies,omitempty"`
	RoleConsumers      []PluginRoleConsumer    `json:"roleConsumers,omitempty"`
}

type PluginFilter struct {
	Kind           PluginKind
	LifecycleState PluginLifecycleState
	SourceType     PluginSourceType
	TrustState     PluginTrustState
}

type PluginRuntimeStatus struct {
	PluginID           string                 `json:"plugin_id"`
	Host               PluginRuntimeHost      `json:"host"`
	LifecycleState     PluginLifecycleState   `json:"lifecycle_state"`
	LastHealthAt       *time.Time             `json:"last_health_at,omitempty"`
	LastError          string                 `json:"last_error,omitempty"`
	RestartCount       int                    `json:"restart_count"`
	ResolvedSourcePath string                 `json:"resolved_source_path,omitempty"`
	RuntimeMetadata    *PluginRuntimeMetadata `json:"runtime_metadata,omitempty"`
}

type PluginInstanceSnapshot struct {
	PluginID           string                 `json:"plugin_id"`
	ProjectID          string                 `json:"project_id,omitempty"`
	RuntimeHost        PluginRuntimeHost      `json:"runtime_host"`
	LifecycleState     PluginLifecycleState   `json:"lifecycle_state"`
	ResolvedSourcePath string                 `json:"resolved_source_path,omitempty"`
	RuntimeMetadata    *PluginRuntimeMetadata `json:"runtime_metadata,omitempty"`
	RestartCount       int                    `json:"restart_count"`
	LastHealthAt       *time.Time             `json:"last_health_at,omitempty"`
	LastError          string                 `json:"last_error,omitempty"`
	CreatedAt          time.Time              `json:"created_at,omitempty"`
	UpdatedAt          time.Time              `json:"updated_at,omitempty"`
}

type PluginEventType string

const (
	PluginEventInstalled      PluginEventType = "installed"
	PluginEventEnabled        PluginEventType = "enabled"
	PluginEventDisabled       PluginEventType = "disabled"
	PluginEventDeactivated    PluginEventType = "deactivated"
	PluginEventActivating     PluginEventType = "activating"
	PluginEventActivated      PluginEventType = "activated"
	PluginEventUpdated        PluginEventType = "updated"
	PluginEventMCPDiscovery   PluginEventType = "mcp_discovery"
	PluginEventMCPInteraction PluginEventType = "mcp_interaction"
	PluginEventRuntimeSync    PluginEventType = "runtime_sync"
	PluginEventHealth         PluginEventType = "health"
	PluginEventRestarted      PluginEventType = "restarted"
	PluginEventInvoked        PluginEventType = "invoked"
	PluginEventUninstalled    PluginEventType = "uninstalled"
	PluginEventFailed         PluginEventType = "failed"
)

type PluginEventSource string

const (
	PluginEventSourceControlPlane PluginEventSource = "control-plane"
	PluginEventSourceTSBridge     PluginEventSource = "ts-bridge"
	PluginEventSourceGoRuntime    PluginEventSource = "go-runtime"
	PluginEventSourceOperator     PluginEventSource = "operator"
)

type PluginEventRecord struct {
	ID             string               `json:"id"`
	PluginID       string               `json:"plugin_id"`
	EventType      PluginEventType      `json:"event_type"`
	EventSource    PluginEventSource    `json:"event_source"`
	LifecycleState PluginLifecycleState `json:"lifecycle_state,omitempty"`
	Summary        string               `json:"summary,omitempty"`
	Payload        map[string]any       `json:"payload,omitempty"`
	CreatedAt      time.Time            `json:"created_at,omitempty"`
}

// UpdatePluginConfigRequest is the request body for updating plugin configuration.
type UpdatePluginConfigRequest struct {
	Config map[string]interface{} `json:"config" validate:"required"`
}

// MarketplacePluginDTO represents a plugin available in the marketplace.
type MarketplacePluginDTO struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Version       string                 `json:"version"`
	Author        string                 `json:"author"`
	Kind          string                 `json:"kind"`
	InstallURL    string                 `json:"installUrl"`
	Installed     bool                   `json:"installed"`
	SourceType    string                 `json:"sourceType,omitempty"`
	Registry      string                 `json:"registry,omitempty"`
	Runtime       string                 `json:"runtime,omitempty"`
	Installable   bool                   `json:"installable,omitempty"`
	BlockedReason string                 `json:"blockedReason,omitempty"`
	TrustStatus   PluginTrustState       `json:"trustStatus,omitempty"`
	ApprovalState PluginApprovalState    `json:"approvalState,omitempty"`
	Release       *PluginReleaseMetadata `json:"release,omitempty"`
	BuiltIn       *PluginBuiltInMetadata `json:"builtIn,omitempty"`
}

type RemoteRegistryErrorCode string

const (
	RemoteRegistryUnconfigured       RemoteRegistryErrorCode = "remote_registry_unconfigured"
	RemoteRegistryUnavailable        RemoteRegistryErrorCode = "remote_registry_unavailable"
	RemoteRegistryDownloadFailed     RemoteRegistryErrorCode = "remote_registry_download_failed"
	RemoteRegistryInvalidArtifact    RemoteRegistryErrorCode = "remote_registry_invalid_artifact"
	RemoteRegistryVerificationFailed RemoteRegistryErrorCode = "remote_registry_verification_failed"
)

type RemoteMarketplaceResponse struct {
	Available bool                    `json:"available"`
	Registry  string                  `json:"registry,omitempty"`
	ErrorCode RemoteRegistryErrorCode `json:"errorCode,omitempty"`
	Error     string                  `json:"error,omitempty"`
	Entries   []MarketplacePluginDTO  `json:"entries"`
}

type RemotePluginInstallResponse struct {
	OK        bool                    `json:"ok"`
	PluginID  string                  `json:"pluginId"`
	Version   string                  `json:"version,omitempty"`
	Registry  string                  `json:"registry,omitempty"`
	ErrorCode RemoteRegistryErrorCode `json:"errorCode,omitempty"`
	Message   string                  `json:"message,omitempty"`
}
