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
	PluginRuntimeDeclarative PluginRuntime = "declarative"
	PluginRuntimeMCP         PluginRuntime = "mcp"
	PluginRuntimeGoPlugin    PluginRuntime = "go-plugin"
)

type PluginSourceType string

const (
	PluginSourceBuiltin PluginSourceType = "builtin"
	PluginSourceLocal   PluginSourceType = "local"
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
	Runtime   PluginRuntime   `yaml:"runtime" json:"runtime"`
	Transport string          `yaml:"transport,omitempty" json:"transport,omitempty"`
	Command   string          `yaml:"command,omitempty" json:"command,omitempty"`
	Args      []string        `yaml:"args,omitempty" json:"args,omitempty"`
	URL       string          `yaml:"url,omitempty" json:"url,omitempty"`
	Binary    string          `yaml:"binary,omitempty" json:"binary,omitempty"`
	Config    map[string]any  `yaml:"config,omitempty" json:"config,omitempty"`
	Extra     map[string]any  `yaml:",inline" json:"extra,omitempty"`
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
	Type PluginSourceType `yaml:"type,omitempty" json:"type"`
	Path string           `yaml:"path,omitempty" json:"path,omitempty"`
}

type PluginRecord struct {
	PluginManifest
	LifecycleState PluginLifecycleState `json:"lifecycle_state"`
	RuntimeHost    PluginRuntimeHost    `json:"runtime_host,omitempty"`
	LastHealthAt   *time.Time           `json:"last_health_at,omitempty"`
	LastError      string               `json:"last_error,omitempty"`
	RestartCount   int                  `json:"restart_count"`
}

type PluginFilter struct {
	Kind           PluginKind
	LifecycleState PluginLifecycleState
}

type PluginRuntimeStatus struct {
	PluginID       string               `json:"plugin_id"`
	Host           PluginRuntimeHost    `json:"host"`
	LifecycleState PluginLifecycleState `json:"lifecycle_state"`
	LastHealthAt   *time.Time           `json:"last_health_at,omitempty"`
	LastError      string               `json:"last_error,omitempty"`
	RestartCount   int                  `json:"restart_count"`
}
