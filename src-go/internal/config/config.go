// Package config loads application configuration from environment variables and .env files.
package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Port           string
	PostgresURL    string
	RedisURL       string
	JWTSecret      string
	JWTAccessTTL   time.Duration
	JWTRefreshTTL  time.Duration
	AllowOrigins   []string
	Env            string
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

	accessTTL, _ := time.ParseDuration(viper.GetString("JWT_ACCESS_TTL"))
	refreshTTL, _ := time.ParseDuration(viper.GetString("JWT_REFRESH_TTL"))

	origins := strings.Split(viper.GetString("ALLOW_ORIGINS"), ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}

	return &Config{
		Port:          viper.GetString("PORT"),
		PostgresURL:   viper.GetString("POSTGRES_URL"),
		RedisURL:      viper.GetString("REDIS_URL"),
		JWTSecret:     viper.GetString("JWT_SECRET"),
		JWTAccessTTL:  accessTTL,
		JWTRefreshTTL: refreshTTL,
		AllowOrigins:  origins,
		Env:           viper.GetString("ENV"),
	}
}
