package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
)

type codingAgentRuntimeSpec struct {
	runtime             string
	label               string
	defaultProvider     string
	compatibleProviders []string
	defaultModel        string
}

var codingAgentRuntimeSpecs = map[string]codingAgentRuntimeSpec{
	"claude_code": {
		runtime:             "claude_code",
		label:               "Claude Code",
		defaultProvider:     "anthropic",
		compatibleProviders: []string{"anthropic"},
		defaultModel:        "claude-sonnet-4-5",
	},
	"codex": {
		runtime:             "codex",
		label:               "Codex",
		defaultProvider:     "openai",
		compatibleProviders: []string{"openai", "codex"},
		defaultModel:        "gpt-5-codex",
	},
	"opencode": {
		runtime:             "opencode",
		label:               "OpenCode",
		defaultProvider:     "opencode",
		compatibleProviders: []string{"opencode"},
		defaultModel:        "opencode-default",
	},
}

func ResolveProjectCodingAgentSelection(
	project *model.Project,
	runtime string,
	provider string,
	modelName string,
) (model.CodingAgentSelection, error) {
	settings := model.ProjectStoredSettings{}
	if project != nil {
		settings = project.StoredSettings()
	}

	resolvedRuntime := normalizeRuntime(runtime)
	if resolvedRuntime == "" {
		resolvedRuntime = normalizeRuntime(settings.CodingAgent.Runtime)
	}
	if resolvedRuntime == "" {
		resolvedRuntime = model.DefaultCodingAgentRuntime
	}

	spec, ok := codingAgentRuntimeSpecs[resolvedRuntime]
	if !ok {
		return model.CodingAgentSelection{}, fmt.Errorf("unsupported coding-agent runtime: %s", resolvedRuntime)
	}

	resolvedProvider := normalizeProvider(provider)
	if resolvedProvider == "" {
		resolvedProvider = normalizeProvider(settings.CodingAgent.Provider)
	}
	if resolvedProvider == "" {
		resolvedProvider = spec.defaultProvider
	}
	if !providerCompatible(spec, resolvedProvider) {
		return model.CodingAgentSelection{}, fmt.Errorf("runtime %s is incompatible with provider %s", spec.runtime, resolvedProvider)
	}

	resolvedModel := strings.TrimSpace(modelName)
	if resolvedModel == "" {
		resolvedModel = strings.TrimSpace(settings.CodingAgent.Model)
	}
	if resolvedModel == "" {
		resolvedModel = spec.defaultModel
	}
	if resolvedModel == "" {
		return model.CodingAgentSelection{}, fmt.Errorf("runtime %s does not have a default model", spec.runtime)
	}

	return model.CodingAgentSelection{
		Runtime:  spec.runtime,
		Provider: resolvedProvider,
		Model:    resolvedModel,
	}, nil
}

func MarshalCodingAgentSelection(selection model.CodingAgentSelection) string {
	payload, err := json.Marshal(selection)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func DefaultCodingAgentCatalog(selection model.CodingAgentSelection) *model.CodingAgentCatalogDTO {
	runtimes := make([]model.CodingAgentRuntimeOptionDTO, 0, len(codingAgentRuntimeSpecs))
	order := []string{"claude_code", "codex", "opencode"}
	for _, key := range order {
		spec := codingAgentRuntimeSpecs[key]
		runtimes = append(runtimes, model.CodingAgentRuntimeOptionDTO{
			Runtime:             spec.runtime,
			Label:               spec.label,
			DefaultProvider:     spec.defaultProvider,
			CompatibleProviders: append([]string(nil), spec.compatibleProviders...),
			DefaultModel:        spec.defaultModel,
			Available:           true,
			Diagnostics:         []model.CodingAgentDiagnosticDTO{},
		})
	}
	return &model.CodingAgentCatalogDTO{
		DefaultRuntime:   model.DefaultCodingAgentRuntime,
		DefaultSelection: selection,
		Runtimes:         runtimes,
	}
}

func normalizeRuntime(runtime string) string {
	return strings.TrimSpace(strings.ToLower(runtime))
}

func normalizeProvider(provider string) string {
	return strings.TrimSpace(strings.ToLower(provider))
}

func providerCompatible(spec codingAgentRuntimeSpec, provider string) bool {
	for _, candidate := range spec.compatibleProviders {
		if candidate == provider {
			return true
		}
	}
	return false
}
