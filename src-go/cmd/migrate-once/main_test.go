package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMainRequiresPostgresURL(t *testing.T) {
	result := runMigrateOnceHelper(t, "missing-postgres-url", map[string]string{
		"POSTGRES_URL": "",
	})
	if result.exitCode == 0 {
		t.Fatal("expected non-zero exit code when POSTGRES_URL is missing")
	}
	if !strings.Contains(result.stderr, "POSTGRES_URL is required") {
		t.Fatalf("stderr = %q, want POSTGRES_URL requirement", result.stderr)
	}
}

func TestMainRejectsInvalidMigrationURL(t *testing.T) {
	result := runMigrateOnceHelper(t, "invalid-migration-url", map[string]string{
		"POSTGRES_URL": "postgres://invalid:invalid@127.0.0.1:59999/noexist?connect_timeout=1",
	})
	if result.exitCode == 0 {
		t.Fatal("expected non-zero exit code for invalid migration URL")
	}
	if !strings.Contains(result.stderr, "migration init failed") && !strings.Contains(result.stderr, "migration run failed") {
		t.Fatalf("stderr = %q, want migration init/run failure", result.stderr)
	}
}

func TestMigrateOnceHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	switch os.Getenv("MIGRATE_ONCE_CASE") {
	case "missing-postgres-url":
		_ = os.Unsetenv("POSTGRES_URL")
	case "invalid-migration-url":
		if err := os.Setenv("POSTGRES_URL", "postgres://invalid:invalid@127.0.0.1:59999/noexist?connect_timeout=1"); err != nil {
			t.Fatalf("Setenv(POSTGRES_URL) error = %v", err)
		}
	default:
		t.Fatalf("unknown helper case %q", os.Getenv("MIGRATE_ONCE_CASE"))
	}

	main()
}

type helperResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func runMigrateOnceHelper(t *testing.T, helperCase string, env map[string]string) helperResult {
	t.Helper()

	command := exec.Command(os.Args[0], "-test.run=TestMigrateOnceHelperProcess")
	command.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"MIGRATE_ONCE_CASE="+helperCase,
	)
	for key, value := range env {
		command.Env = append(command.Env, key+"="+value)
	}

	stdout, err := command.Output()
	result := helperResult{
		stdout: string(stdout),
	}
	if err == nil {
		return result
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("helper command error = %v", err)
	}
	result.exitCode = exitErr.ExitCode()
	result.stderr = string(exitErr.Stderr)
	return result
}
