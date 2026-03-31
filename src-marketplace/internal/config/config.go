// Package config loads application configuration from environment variables and .env files.
package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration values for the marketplace service.
type Config struct {
	Port         string
	Env          string
	PostgresURL  string
	JWTSecret    string
	AllowOrigins []string
	ArtifactsDir string
	AdminUserIDs []string
	MaxUploadMB  int64
}

// Load reads configuration from environment variables and an optional .env file.
func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()

	viper.SetDefault("PORT", "7779")
	viper.SetDefault("ENV", "development")
	viper.SetDefault("ARTIFACTS_DIR", "./data/artifacts")
	viper.SetDefault("MAX_UPLOAD_MB", 100)
	viper.SetDefault("ADMIN_USER_IDS", "")
	viper.SetDefault("ALLOW_ORIGINS", "http://localhost:3000,http://localhost:1420,tauri://localhost")

	adminStr := viper.GetString("ADMIN_USER_IDS")
	var adminIDs []string
	if adminStr != "" {
		for _, id := range strings.Split(adminStr, ",") {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				adminIDs = append(adminIDs, trimmed)
			}
		}
	}

	originsStr := viper.GetString("ALLOW_ORIGINS")
	var origins []string
	for _, o := range strings.Split(originsStr, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return &Config{
		Port:         viper.GetString("PORT"),
		Env:          viper.GetString("ENV"),
		PostgresURL:  viper.GetString("POSTGRES_URL"),
		JWTSecret:    viper.GetString("JWT_SECRET"),
		AllowOrigins: origins,
		ArtifactsDir: viper.GetString("ARTIFACTS_DIR"),
		AdminUserIDs: adminIDs,
		MaxUploadMB:  viper.GetInt64("MAX_UPLOAD_MB"),
	}, nil
}
