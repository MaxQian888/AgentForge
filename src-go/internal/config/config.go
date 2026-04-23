// Package config loads application configuration from environment variables and .env files.
package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Port                         string
	PostgresURL                  string
	RedisURL                     string
	JWTSecret                    string
	JWTAccessTTL                 time.Duration
	JWTRefreshTTL                time.Duration
	AllowOrigins                 []string
	Env                          string
	BridgeURL                    string
	AgentForgeToken              string
	AgentDefaultMaxTurns         int
	WSMaxMessageSizeBytes        int64
	DashboardWidgetCacheTTL      time.Duration
	WorktreeBasePath             string
	RepoBasePath                 string
	RolesDir                     string
	PluginsDir                   string
	PluginRegistryURL            string
	BridgeToolManifestAllowlist  []string
	SchedulerExecutionMode       string
	MaxActiveAgents              int
	DefaultTaskBudget            float64
	TaskProgressWarningAfter     time.Duration
	TaskProgressStalledAfter     time.Duration
	TaskProgressAlertCooldown    time.Duration
	TaskProgressDetectorInterval time.Duration
	TaskProgressExemptStatuses   []string
	IMNotifyURL                  string
	IMNotifyPlatform             string
	IMNotifyTargetChatID         string
	IMControlSharedSecret        string
	IMBridgeHeartbeatTTL         time.Duration
	IMBridgeProgressInterval     time.Duration
	MarketplaceURL               string
	// FrontendAcceptInvitationURL is the absolute or relative URL that
	// accept-invitation tokens are appended to. Used in delivery messages
	// and the Create response's `acceptUrl` field.
	FrontendAcceptInvitationURL string
	// UseWorkflowBackedReview enables the experimental path in ReviewService.Trigger
	// that launches a system:code-review workflow execution in parallel with the
	// legacy bridge-based review. Off by default.
	UseWorkflowBackedReview bool
	// PublicBaseURL is the externally reachable base URL of the AgentForge
	// backend (e.g. https://agentforge.acme.corp). Used to compute webhook
	// callback URLs handed to VCS hosts. Empty in dev triggers a startup
	// warning; production deployments MUST set it.
	PublicBaseURL string
	// FrontendBaseURL is the canonical FE origin used by the
	// outbound_dispatcher (spec §5) to construct workflow run links inside
	// default reply cards (`<base>/runs/<exec_id>`). Defaults to
	// http://localhost:3000.
	FrontendBaseURL string
	// LogLevel overrides the env-based log level when set.
	// Accepted values: debug, info, warn, error. Empty = env-based default.
	LogLevel string
}

func Load() *Config {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Load .env file if it exists (ignore error if missing)
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	_ = viper.ReadInConfig()

	// Defaults
	viper.SetDefault("PORT", "7777")
	viper.SetDefault("ENV", "development")
	viper.SetDefault("JWT_ACCESS_TTL", "15m")
	viper.SetDefault("JWT_REFRESH_TTL", "168h")
	viper.SetDefault("ALLOW_ORIGINS", "http://localhost:3000,tauri://localhost,http://localhost:1420")
	viper.SetDefault("REDIS_URL", "redis://localhost:6379")
	viper.SetDefault("BRIDGE_URL", "http://localhost:7778")
	viper.SetDefault("AGENTFORGE_TOKEN", "")
	viper.SetDefault("AGENT_DEFAULT_MAX_TURNS", 50)
	viper.SetDefault("WS_MAX_MESSAGE_SIZE_BYTES", 4096)
	viper.SetDefault("DASHBOARD_WIDGET_CACHE_TTL", "60s")
	viper.SetDefault("WORKTREE_BASE_PATH", "./data/worktrees")
	viper.SetDefault("REPO_BASE_PATH", "./data/repos")
	viper.SetDefault("ROLES_DIR", "./roles")
	viper.SetDefault("PLUGINS_DIR", "./plugins")
	viper.SetDefault("PLUGIN_REGISTRY_URL", "")
	viper.SetDefault("BRIDGE_TOOL_MANIFEST_ALLOWLIST", "")
	viper.SetDefault("SCHEDULER_EXECUTION_MODE", "in_process")
	viper.SetDefault("MAX_ACTIVE_AGENTS", 20)
	viper.SetDefault("DEFAULT_TASK_BUDGET", 5.0)
	viper.SetDefault("TASK_PROGRESS_WARNING_AFTER", "2h")
	viper.SetDefault("TASK_PROGRESS_STALLED_AFTER", "4h")
	viper.SetDefault("TASK_PROGRESS_ALERT_COOLDOWN", "30m")
	viper.SetDefault("TASK_PROGRESS_DETECTOR_INTERVAL", "1m")
	viper.SetDefault("TASK_PROGRESS_EXEMPT_STATUSES", "blocked,done,cancelled")
	viper.SetDefault("IM_NOTIFY_URL", "")
	viper.SetDefault("IM_NOTIFY_PLATFORM", "")
	viper.SetDefault("IM_NOTIFY_TARGET_CHAT_ID", "")
	viper.SetDefault("IM_CONTROL_SHARED_SECRET", "")
	viper.SetDefault("IM_BRIDGE_HEARTBEAT_TTL", "2m")
	viper.SetDefault("IM_BRIDGE_PROGRESS_INTERVAL", "30s")
	viper.SetDefault("MARKETPLACE_URL", "http://localhost:7781")
	viper.SetDefault("FRONTEND_ACCEPT_INVITATION_URL", "http://localhost:3000/invitations/accept")
	viper.SetDefault("USE_WORKFLOW_BACKED_REVIEW", false)
	viper.SetDefault("AGENTFORGE_PUBLIC_BASE_URL", "")
	viper.SetDefault("FRONTEND_BASE_URL", "http://localhost:3000")

	accessTTL, _ := time.ParseDuration(viper.GetString("JWT_ACCESS_TTL"))
	refreshTTL, _ := time.ParseDuration(viper.GetString("JWT_REFRESH_TTL"))
	taskProgressWarningAfter, _ := time.ParseDuration(viper.GetString("TASK_PROGRESS_WARNING_AFTER"))
	taskProgressStalledAfter, _ := time.ParseDuration(viper.GetString("TASK_PROGRESS_STALLED_AFTER"))
	taskProgressAlertCooldown, _ := time.ParseDuration(viper.GetString("TASK_PROGRESS_ALERT_COOLDOWN"))
	taskProgressDetectorInterval, _ := time.ParseDuration(viper.GetString("TASK_PROGRESS_DETECTOR_INTERVAL"))
	dashboardWidgetCacheTTL, _ := time.ParseDuration(viper.GetString("DASHBOARD_WIDGET_CACHE_TTL"))
	imBridgeHeartbeatTTL, _ := time.ParseDuration(viper.GetString("IM_BRIDGE_HEARTBEAT_TTL"))
	imBridgeProgressInterval, _ := time.ParseDuration(viper.GetString("IM_BRIDGE_PROGRESS_INTERVAL"))

	origins := strings.Split(viper.GetString("ALLOW_ORIGINS"), ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}
	exemptStatuses := strings.Split(viper.GetString("TASK_PROGRESS_EXEMPT_STATUSES"), ",")
	for i, status := range exemptStatuses {
		exemptStatuses[i] = strings.TrimSpace(status)
	}
	bridgeToolManifestAllowlist := strings.Split(viper.GetString("BRIDGE_TOOL_MANIFEST_ALLOWLIST"), ",")
	for i, host := range bridgeToolManifestAllowlist {
		bridgeToolManifestAllowlist[i] = strings.TrimSpace(host)
	}

	return &Config{
		Port:                         viper.GetString("PORT"),
		PostgresURL:                  viper.GetString("POSTGRES_URL"),
		RedisURL:                     viper.GetString("REDIS_URL"),
		JWTSecret:                    viper.GetString("JWT_SECRET"),
		JWTAccessTTL:                 accessTTL,
		JWTRefreshTTL:                refreshTTL,
		AllowOrigins:                 origins,
		Env:                          viper.GetString("ENV"),
		BridgeURL:                    viper.GetString("BRIDGE_URL"),
		AgentForgeToken:              viper.GetString("AGENTFORGE_TOKEN"),
		AgentDefaultMaxTurns:         viper.GetInt("AGENT_DEFAULT_MAX_TURNS"),
		WSMaxMessageSizeBytes:        viper.GetInt64("WS_MAX_MESSAGE_SIZE_BYTES"),
		DashboardWidgetCacheTTL:      dashboardWidgetCacheTTL,
		WorktreeBasePath:             viper.GetString("WORKTREE_BASE_PATH"),
		RepoBasePath:                 viper.GetString("REPO_BASE_PATH"),
		RolesDir:                     viper.GetString("ROLES_DIR"),
		PluginsDir:                   viper.GetString("PLUGINS_DIR"),
		PluginRegistryURL:            viper.GetString("PLUGIN_REGISTRY_URL"),
		BridgeToolManifestAllowlist:  bridgeToolManifestAllowlist,
		SchedulerExecutionMode:       viper.GetString("SCHEDULER_EXECUTION_MODE"),
		MaxActiveAgents:              viper.GetInt("MAX_ACTIVE_AGENTS"),
		DefaultTaskBudget:            viper.GetFloat64("DEFAULT_TASK_BUDGET"),
		TaskProgressWarningAfter:     taskProgressWarningAfter,
		TaskProgressStalledAfter:     taskProgressStalledAfter,
		TaskProgressAlertCooldown:    taskProgressAlertCooldown,
		TaskProgressDetectorInterval: taskProgressDetectorInterval,
		TaskProgressExemptStatuses:   exemptStatuses,
		IMNotifyURL:                  viper.GetString("IM_NOTIFY_URL"),
		IMNotifyPlatform:             viper.GetString("IM_NOTIFY_PLATFORM"),
		IMNotifyTargetChatID:         viper.GetString("IM_NOTIFY_TARGET_CHAT_ID"),
		IMControlSharedSecret:        viper.GetString("IM_CONTROL_SHARED_SECRET"),
		IMBridgeHeartbeatTTL:         imBridgeHeartbeatTTL,
		IMBridgeProgressInterval:     imBridgeProgressInterval,
		MarketplaceURL:               viper.GetString("MARKETPLACE_URL"),
		FrontendAcceptInvitationURL:  viper.GetString("FRONTEND_ACCEPT_INVITATION_URL"),
		UseWorkflowBackedReview:      viper.GetBool("USE_WORKFLOW_BACKED_REVIEW"),
		PublicBaseURL:                viper.GetString("AGENTFORGE_PUBLIC_BASE_URL"),
		FrontendBaseURL:              viper.GetString("FRONTEND_BASE_URL"),
		LogLevel:                     viper.GetString("LOG_LEVEL"),
	}
}
