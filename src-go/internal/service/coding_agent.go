package service

import (
	_ "embed"
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
	modelOptions        []string
	supportedFeatures   []string
	strictModelOptions  bool
}

type codingAgentRuntimeProfileDocument struct {
	Key                 string   `json:"key"`
	Label               string   `json:"label"`
	DefaultProvider     string   `json:"default_provider"`
	CompatibleProviders []string `json:"compatible_providers"`
	DefaultModel        string   `json:"default_model"`
	ModelOptions        []string `json:"model_options"`
	StrictModelOptions  bool     `json:"strict_model_options"`
	SupportedFeatures   []string `json:"supported_features"`
}

//go:embed coding_agent_backend_profiles.json
var codingAgentRuntimeProfilesJSON []byte

var (
	codingAgentRuntimeSpecs  = loadCodingAgentRuntimeSpecs()
	codingAgentRuntimeOrders = loadCodingAgentRuntimeOrder()
)

func loadCodingAgentRuntimeDocuments() []codingAgentRuntimeProfileDocument {
	var docs []codingAgentRuntimeProfileDocument
	if err := json.Unmarshal(codingAgentRuntimeProfilesJSON, &docs); err != nil {
		panic(fmt.Sprintf("decode coding agent runtime profiles: %v", err))
	}
	return docs
}

func loadCodingAgentRuntimeSpecs() map[string]codingAgentRuntimeSpec {
	docs := loadCodingAgentRuntimeDocuments()
	specs := make(map[string]codingAgentRuntimeSpec, len(docs))
	for _, doc := range docs {
		specs[doc.Key] = codingAgentRuntimeSpec{
			runtime:             doc.Key,
			label:               doc.Label,
			defaultProvider:     doc.DefaultProvider,
			compatibleProviders: append([]string(nil), doc.CompatibleProviders...),
			defaultModel:        doc.DefaultModel,
			modelOptions:        append([]string(nil), doc.ModelOptions...),
			supportedFeatures:   append([]string(nil), doc.SupportedFeatures...),
			strictModelOptions:  doc.StrictModelOptions,
		}
	}
	return specs
}

func loadCodingAgentRuntimeOrder() []string {
	docs := loadCodingAgentRuntimeDocuments()
	order := make([]string, 0, len(docs))
	for _, doc := range docs {
		order = append(order, doc.Key)
	}
	return order
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
	if spec.strictModelOptions && len(spec.modelOptions) > 0 && !modelCompatible(spec, resolvedModel) {
		return model.CodingAgentSelection{}, fmt.Errorf("runtime %s does not support model %s", spec.runtime, resolvedModel)
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
	for _, key := range codingAgentRuntimeOrders {
		spec := codingAgentRuntimeSpecs[key]
		runtimes = append(runtimes, model.CodingAgentRuntimeOptionDTO{
			Runtime:             spec.runtime,
			Label:               spec.label,
			DefaultProvider:     spec.defaultProvider,
			CompatibleProviders: append([]string(nil), spec.compatibleProviders...),
			DefaultModel:        spec.defaultModel,
			ModelOptions:        append([]string(nil), spec.modelOptions...),
			Available:           true,
			Diagnostics:         []model.CodingAgentDiagnosticDTO{},
			SupportedFeatures:   append([]string(nil), spec.supportedFeatures...),
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

func modelCompatible(spec codingAgentRuntimeSpec, modelName string) bool {
	for _, candidate := range spec.modelOptions {
		if candidate == modelName {
			return true
		}
	}
	return false
}
