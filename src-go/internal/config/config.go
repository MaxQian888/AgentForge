// Package config loads application configuration from environment variables and .env files.
package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Port              string
	PostgresURL       string
	RedisURL          string
	JWTSecret         string
	JWTAccessTTL      time.Duration
	JWTRefreshTTL     time.Duration
	AllowOrigins      []string
	Env               string
	BridgeURL         string
	WorktreeBasePath  string
	RepoBasePath      string
	RolesDir          string
	MaxActiveAgents   int
	DefaultTaskBudget float64
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
	viper.SetDefault("WORKTREE_BASE_PATH", "./data/worktrees")
	viper.SetDefault("REPO_BASE_PATH", "./data/repos")
	viper.SetDefault("ROLES_DIR", "./roles")
	viper.SetDefault("MAX_ACTIVE_AGENTS", 20)
	viper.SetDefault("DEFAULT_TASK_BUDGET", 5.0)

	accessTTL, _ := time.ParseDuration(viper.GetString("JWT_ACCESS_TTL"))
	refreshTTL, _ := time.ParseDuration(viper.GetString("JWT_REFRESH_TTL"))

	origins := strings.Split(viper.GetString("ALLOW_ORIGINS"), ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}

	return &Config{
		Port:              viper.GetString("PORT"),
		PostgresURL:       viper.GetString("POSTGRES_URL"),
		RedisURL:          viper.GetString("REDIS_URL"),
		JWTSecret:         viper.GetString("JWT_SECRET"),
		JWTAccessTTL:      accessTTL,
		JWTRefreshTTL:     refreshTTL,
		AllowOrigins:      origins,
		Env:               viper.GetString("ENV"),
		BridgeURL:         viper.GetString("BRIDGE_URL"),
		WorktreeBasePath:  viper.GetString("WORKTREE_BASE_PATH"),
		RepoBasePath:      viper.GetString("REPO_BASE_PATH"),
		RolesDir:          viper.GetString("ROLES_DIR"),
		MaxActiveAgents:   viper.GetInt("MAX_ACTIVE_AGENTS"),
		DefaultTaskBudget: viper.GetFloat64("DEFAULT_TASK_BUDGET"),
	}
}
