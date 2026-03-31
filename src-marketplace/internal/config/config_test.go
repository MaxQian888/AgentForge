package config_test

import (
	"os"
	"testing"

	"github.com/agentforge/marketplace/internal/config"
)

func TestConfig_DefaultPort(t *testing.T) {
	// Clear any previously-set value so the default kicks in.
	os.Unsetenv("PORT")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.Port != "7779" {
		t.Errorf("expected default port 7779, got %q", cfg.Port)
	}
}

func TestConfig_DefaultArtifactsDir(t *testing.T) {
	os.Unsetenv("ARTIFACTS_DIR")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.ArtifactsDir == "" {
		t.Error("ArtifactsDir should have a default value")
	}
}

func TestConfig_DefaultMaxUploadMB(t *testing.T) {
	os.Unsetenv("MAX_UPLOAD_MB")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.MaxUploadMB != 100 {
		t.Errorf("expected default MaxUploadMB 100, got %d", cfg.MaxUploadMB)
	}
}

func TestConfig_EnvOverride(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("ENV", "production")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.Port != "9999" {
		t.Errorf("expected port 9999, got %q", cfg.Port)
	}
	if cfg.Env != "production" {
		t.Errorf("expected env production, got %q", cfg.Env)
	}
}

func TestConfig_AllowOriginsDefault(t *testing.T) {
	os.Unsetenv("ALLOW_ORIGINS")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if len(cfg.AllowOrigins) == 0 {
		t.Error("AllowOrigins should have default values")
	}
}

func TestConfig_AdminUserIDsParsing(t *testing.T) {
	t.Setenv("ADMIN_USER_IDS", "uuid-1, uuid-2 , uuid-3")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if len(cfg.AdminUserIDs) != 3 {
		t.Errorf("expected 3 admin IDs, got %d", len(cfg.AdminUserIDs))
	}
	// Verify whitespace is trimmed.
	for _, id := range cfg.AdminUserIDs {
		if id != "uuid-1" && id != "uuid-2" && id != "uuid-3" {
			t.Errorf("unexpected admin ID %q (whitespace not trimmed?)", id)
		}
	}
}

func TestConfig_EmptyAdminUserIDs(t *testing.T) {
	t.Setenv("ADMIN_USER_IDS", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if len(cfg.AdminUserIDs) != 0 {
		t.Errorf("expected empty AdminUserIDs, got %v", cfg.AdminUserIDs)
	}
}
