package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDecodeObjectHandlesEmptyNullAndInvalidJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]any
		wantErr bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  map[string]any{},
		},
		{
			name:  "json null",
			input: "null",
			want:  map[string]any{},
		},
		{
			name:  "object",
			input: `{"mode":"webhook","retries":2}`,
			want: map[string]any{
				"mode":    "webhook",
				"retries": float64(2),
			},
		},
		{
			name:    "invalid",
			input:   "{oops",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeObject(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("decodeObject() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeObject() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("decodeObject() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestMergeConfigOverlaysOverrides(t *testing.T) {
	base := map[string]any{
		"mode":   "webhook",
		"region": "cn",
	}
	override := map[string]any{
		"mode":  "debug",
		"token": "secret",
	}

	got := mergeConfig(base, override)
	want := map[string]any{
		"mode":   "debug",
		"region": "cn",
		"token":  "secret",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeConfig() = %#v, want %#v", got, want)
	}

	got = mergeConfig(nil, nil)
	if !reflect.DeepEqual(got, map[string]any{}) {
		t.Fatalf("mergeConfig(nil, nil) = %#v, want empty map", got)
	}
}

func TestMainDebugHealthOutputsStructuredResult(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "..", "plugins", "integrations", "sample-integration-plugin", "manifest.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest not available: %v", err)
	}

	stdout, stderr := runMain(t, []string{
		"plugin-debugger",
		"--manifest", manifestPath,
		"--operation", "health",
		"--config", `{"mode":"test"}`,
	})
	if stderr != "" {
		t.Fatalf("main() stderr = %q, want empty", stderr)
	}

	var result struct {
		OK        bool           `json:"ok"`
		Operation string         `json:"operation"`
		Data      map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}
	if !result.OK {
		t.Fatalf("result.OK = false, want true; payload = %s", stdout)
	}
	if result.Operation != "health" {
		t.Fatalf("result.Operation = %q, want %q", result.Operation, "health")
	}
	if result.Data["status"] != "ok" {
		t.Fatalf("result.Data[status] = %v, want ok", result.Data["status"])
	}
	if result.Data["mode"] != "test" {
		t.Fatalf("result.Data[mode] = %v, want test", result.Data["mode"])
	}
}

func runMain(t *testing.T, args []string) (string, string) {
	t.Helper()

	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}

	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
	os.Args = args
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
		os.Args = oldArgs
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})

	main()

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	return string(stdout), string(stderr)
}
