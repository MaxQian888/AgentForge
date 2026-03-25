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
	if manifest.Kind == model.PluginKindWorkflow {
		if manifest.Spec.Runtime == model.PluginRuntimeWASM {
			if strings.TrimSpace(manifest.Spec.Module) == "" {
				return fmt.Errorf("workflow plugin runtime wasm requires spec.module")
			}
			if strings.TrimSpace(manifest.Spec.ABIVersion) == "" {
				return fmt.Errorf("workflow plugin runtime wasm requires spec.abiVersion")
			}
		}
		if manifest.Spec.Workflow == nil {
			return fmt.Errorf("workflow plugin requires spec.workflow")
		}
		if strings.TrimSpace(string(manifest.Spec.Workflow.Process)) == "" {
			return fmt.Errorf("workflow plugin requires spec.workflow.process")
		}
		if len(manifest.Spec.Workflow.Steps) == 0 {
			return fmt.Errorf("workflow plugin requires at least one workflow step")
		}
	}
	if manifest.Kind == model.PluginKindReview {
		if manifest.Spec.Review == nil {
			return fmt.Errorf("review plugin requires spec.review")
		}
		if len(manifest.Spec.Review.Triggers.Events) == 0 {
			return fmt.Errorf("review plugin requires at least one trigger event")
		}
		if strings.TrimSpace(manifest.Spec.Review.Output.Format) == "" {
			return fmt.Errorf("review plugin requires spec.review.output.format")
		}
		if manifest.Spec.Review.Output.Format != "findings/v1" {
			return fmt.Errorf("review plugin output format %q is not supported; expected findings/v1", manifest.Spec.Review.Output.Format)
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
		return runtime == model.PluginRuntimeWASM
	case model.PluginKindIntegration:
		return runtime == model.PluginRuntimeGoPlugin || runtime == model.PluginRuntimeWASM
	case model.PluginKindReview:
		return runtime == model.PluginRuntimeMCP
	default:
		return false
	}
}
