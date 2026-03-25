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
	WorktreeBasePath             string
	RepoBasePath                 string
	RolesDir                     string
	PluginsDir                   string
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
	viper.SetDefault("WORKTREE_BASE_PATH", "./data/worktrees")
	viper.SetDefault("REPO_BASE_PATH", "./data/repos")
	viper.SetDefault("ROLES_DIR", "./roles")
	viper.SetDefault("PLUGINS_DIR", "./plugins")
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

	accessTTL, _ := time.ParseDuration(viper.GetString("JWT_ACCESS_TTL"))
	refreshTTL, _ := time.ParseDuration(viper.GetString("JWT_REFRESH_TTL"))
	taskProgressWarningAfter, _ := time.ParseDuration(viper.GetString("TASK_PROGRESS_WARNING_AFTER"))
	taskProgressStalledAfter, _ := time.ParseDuration(viper.GetString("TASK_PROGRESS_STALLED_AFTER"))
	taskProgressAlertCooldown, _ := time.ParseDuration(viper.GetString("TASK_PROGRESS_ALERT_COOLDOWN"))
	taskProgressDetectorInterval, _ := time.ParseDuration(viper.GetString("TASK_PROGRESS_DETECTOR_INTERVAL"))
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
		WorktreeBasePath:             viper.GetString("WORKTREE_BASE_PATH"),
		RepoBasePath:                 viper.GetString("REPO_BASE_PATH"),
		RolesDir:                     viper.GetString("ROLES_DIR"),
		PluginsDir:                   viper.GetString("PLUGINS_DIR"),
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
	}
}
