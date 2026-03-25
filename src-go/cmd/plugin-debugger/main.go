package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/plugin"
)

func main() {
	var (
		manifestPath string
		operation    string
		configJSON   string
		payloadJSON  string
	)

	flag.StringVar(&manifestPath, "manifest", "", "Path to plugin manifest")
	flag.StringVar(&operation, "operation", "health", "Operation to execute")
	flag.StringVar(&configJSON, "config", "", "Optional config override JSON object")
	flag.StringVar(&payloadJSON, "payload", "", "Optional payload JSON object")
	flag.Parse()

	if manifestPath == "" {
		fatalf("manifest path is required")
	}

	manifest, err := plugin.ParseFile(manifestPath)
	if err != nil {
		fatalErr(err)
	}
	manifest.Source.Path = manifestPath

	configOverride, err := decodeObject(configJSON)
	if err != nil {
		fatalErr(fmt.Errorf("decode config override: %w", err))
	}
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		fatalErr(fmt.Errorf("decode payload override: %w", err))
	}

	record := model.PluginRecord{
		PluginManifest: *manifest,
	}
	if len(configOverride) > 0 {
		record.Spec.Config = mergeConfig(record.Spec.Config, configOverride)
	}

	manager := plugin.NewWASMRuntimeManager()
	result, err := manager.DebugExecute(context.Background(), record, operation, payload)
	if err != nil {
		fatalErr(err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fatalErr(err)
	}

	if result != nil && !result.OK {
		os.Exit(1)
	}
}

func decodeObject(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, err
	}
	if decoded == nil {
		return map[string]any{}, nil
	}
	return decoded, nil
}

func mergeConfig(base map[string]any, override map[string]any) map[string]any {
	if len(base) == 0 && len(override) == 0 {
		return map[string]any{}
	}

	merged := make(map[string]any, len(base)+len(override))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func fatalErr(err error) {
	fatalf("%v", err)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
