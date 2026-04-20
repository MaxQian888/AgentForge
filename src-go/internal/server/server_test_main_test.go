package server_test

import (
	"os"
	"testing"
)

// TestMain ensures the secrets subsystem master key is present for the
// duration of the server-test suite. The bootstrap calls log.Fatal when
// AGENTFORGE_SECRETS_KEY is missing — the production fail-fast contract —
// so tests must set it deterministically before any RegisterRoutes call.
func TestMain(m *testing.M) {
	if os.Getenv("AGENTFORGE_SECRETS_KEY") == "" {
		// 32 raw bytes — deterministic test key.
		os.Setenv("AGENTFORGE_SECRETS_KEY", "0123456789abcdef0123456789abcdef")
		defer os.Unsetenv("AGENTFORGE_SECRETS_KEY")
	}
	os.Exit(m.Run())
}
