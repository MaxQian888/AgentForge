package plugin

import (
	"fmt"
	"os"
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
	"gopkg.in/yaml.v3"
)

func ParseFile(path string) (*model.PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plugin manifest %s: %w", path, err)
	}
	return Parse(data)
}

func Parse(data []byte) (*model.PluginManifest, error) {
	var manifest model.PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse plugin manifest: %w", err)
	}
	if err := ValidateManifest(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func ValidateManifest(manifest *model.PluginManifest) error {
	if manifest == nil {
		return fmt.Errorf("plugin manifest is required")
	}
	if strings.TrimSpace(manifest.APIVersion) == "" {
		return fmt.Errorf("plugin apiVersion is required")
	}
	if strings.TrimSpace(string(manifest.Kind)) == "" {
		return fmt.Errorf("plugin kind is required")
	}
	if strings.TrimSpace(manifest.Metadata.ID) == "" {
		return fmt.Errorf("plugin metadata.id is required")
	}
	if strings.TrimSpace(manifest.Metadata.Name) == "" {
		return fmt.Errorf("plugin metadata.name is required")
	}
	if strings.TrimSpace(manifest.Metadata.Version) == "" {
		return fmt.Errorf("plugin metadata.version is required")
	}
	if strings.TrimSpace(string(manifest.Spec.Runtime)) == "" {
		return fmt.Errorf("plugin spec.runtime is required")
	}

	if !isAllowedRuntime(manifest.Kind, manifest.Spec.Runtime) {
		return fmt.Errorf("plugin kind %s does not support runtime %s", manifest.Kind, manifest.Spec.Runtime)
	}

	if manifest.Kind == model.PluginKindTool && manifest.Spec.Transport == "stdio" && strings.TrimSpace(manifest.Spec.Command) == "" {
		return fmt.Errorf("tool plugin stdio runtime requires spec.command")
	}
	if manifest.Kind == model.PluginKindIntegration && manifest.Spec.Runtime == model.PluginRuntimeGoPlugin && strings.TrimSpace(manifest.Spec.Binary) == "" {
		return fmt.Errorf("integration plugin requires spec.binary")
	}
	if manifest.Kind == model.PluginKindIntegration && manifest.Spec.Runtime == model.PluginRuntimeWASM {
		if strings.TrimSpace(manifest.Spec.Module) == "" {
			return fmt.Errorf("integration plugin runtime wasm requires spec.module")
		}
		if strings.TrimSpace(manifest.Spec.ABIVersion) == "" {
			return fmt.Errorf("integration plugin runtime wasm requires spec.abiVersion")
		}
	}

	return nil
}

func isAllowedRuntime(kind model.PluginKind, runtime model.PluginRuntime) bool {
	switch kind {
	case model.PluginKindRole:
		return runtime == model.PluginRuntimeDeclarative
	case model.PluginKindTool:
		return runtime == model.PluginRuntimeMCP
	case model.PluginKindWorkflow:
		return runtime == model.PluginRuntimeGoPlugin
	case model.PluginKindIntegration:
		return runtime == model.PluginRuntimeGoPlugin || runtime == model.PluginRuntimeWASM
	case model.PluginKindReview:
		return runtime == model.PluginRuntimeMCP
	default:
		return false
	}
}
