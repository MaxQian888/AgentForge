// src-go/internal/config/config_test.go
package config_test

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/react-go-quick-starter/server/internal/config"
)

// resetViper clears viper's global state and changes to a temp dir so that
// any src-go/.env file on disk does not pollute the test.
func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir to temp: %v", err)
	}
	t.Cleanup(func() {
		viper.Reset()
		_ = os.Chdir(orig)
	})
}

func TestLoad_Defaults(t *testing.T) {
	resetViper(t)

	cfg := config.Load()

	if cfg.Port != "7777" {
		t.Errorf("Port: want 7777, got %s", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("Env: want development, got %s", cfg.Env)
	}
	if cfg.JWTAccessTTL.Minutes() != 15 {
		t.Errorf("JWTAccessTTL: want 15m, got %v", cfg.JWTAccessTTL)
	}
	if cfg.JWTRefreshTTL.Hours() != 168 {
		t.Errorf("JWTRefreshTTL: want 168h, got %v", cfg.JWTRefreshTTL)
	}
	if len(cfg.AllowOrigins) == 0 {
		t.Error("AllowOrigins: want non-empty slice, got empty")
	}
	if cfg.PluginsDir != "./plugins" {
		t.Errorf("PluginsDir: want ./plugins, got %s", cfg.PluginsDir)
	}
}

func TestLoad_PortEnvOverride(t *testing.T) {
	resetViper(t)
	t.Setenv("PORT", "9999")

	cfg := config.Load()
	if cfg.Port != "9999" {
		t.Errorf("Port: want 9999 after env override, got %s", cfg.Port)
	}
}

func TestLoad_RedisURLDefault(t *testing.T) {
	resetViper(t)

	cfg := config.Load()
	if cfg.RedisURL == "" {
		t.Error("RedisURL: want non-empty default, got empty")
	}
}
