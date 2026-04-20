package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/agentforge/server/internal/model"
	"gopkg.in/yaml.v3"
)

const builtInBundleFileName = "builtin-bundle.yaml"

type builtInBundleFile struct {
	Plugins []builtInBundleEntry `yaml:"plugins"`
}

type builtInBundleEntry struct {
	ID                  string                  `yaml:"id"`
	Kind                model.PluginKind        `yaml:"kind"`
	Manifest            string                  `yaml:"manifest"`
	DocsRef             string                  `yaml:"docsRef"`
	VerificationProfile string                  `yaml:"verificationProfile"`
	CoreFlows           []string                `yaml:"coreFlows"`
	StarterFamily       string                  `yaml:"starterFamily"`
	DependencyRefs      []string                `yaml:"dependencyRefs"`
	WorkspaceRefs       []string                `yaml:"workspaceRefs"`
	Availability        builtInBundleMetadata   `yaml:"availability"`
	Readiness           *builtInBundleReadiness `yaml:"readiness"`
}

type builtInBundleMetadata struct {
	Status  string `yaml:"status"`
	Message string `yaml:"message"`
}

type builtInBundleReadiness struct {
	ReadyMessage   string                     `yaml:"readyMessage"`
	BlockedMessage string                     `yaml:"blockedMessage"`
	NextStep       string                     `yaml:"nextStep"`
	Installable    *bool                      `yaml:"installable"`
	SupportedHosts []string                   `yaml:"supportedHosts"`
	Prerequisites  []builtInBundleRequirement `yaml:"prerequisites"`
	Configuration  []builtInBundleRequirement `yaml:"configuration"`
}

type builtInBundleRequirement struct {
	Kind  string `yaml:"kind"`
	Value string `yaml:"value"`
	Label string `yaml:"label"`
}

type builtInBundleIndex struct {
	byManifest map[string]builtInBundleEntry
}

func loadBuiltInBundle(root string) (*builtInBundleIndex, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}

	bundlePath := filepath.Join(root, builtInBundleFileName)
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read built-in plugin bundle: %w", err)
	}

	var file builtInBundleFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse built-in plugin bundle: %w", err)
	}

	index := &builtInBundleIndex{byManifest: make(map[string]builtInBundleEntry, len(file.Plugins))}
	for _, entry := range file.Plugins {
		key := normalizeBundleManifestPath(entry.Manifest)
		if key == "" {
			continue
		}
		index.byManifest[key] = entry
	}
	return index, nil
}

func normalizeBundleManifestPath(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	value = strings.TrimPrefix(value, "./")
	value = strings.TrimPrefix(value, "plugins/")
	return strings.TrimPrefix(pathClean(value), "./")
}

func pathClean(value string) string {
	cleaned := filepath.ToSlash(filepath.Clean(value))
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func relativeBundleManifestPath(root, manifestPath string) string {
	if strings.TrimSpace(manifestPath) == "" {
		return ""
	}

	if filepath.IsAbs(manifestPath) {
		rel, err := filepath.Rel(root, manifestPath)
		if err != nil {
			return ""
		}
		return normalizeBundleManifestPath(rel)
	}
	return normalizeBundleManifestPath(manifestPath)
}

func isBuiltInBundleFamily(relativePath string) bool {
	normalized := normalizeBundleManifestPath(relativePath)
	if normalized == "" {
		return false
	}
	head := normalized
	if index := strings.IndexRune(normalized, '/'); index >= 0 {
		head = normalized[:index]
	}
	switch head {
	case "tools", "integrations", "reviews", "workflows":
		return true
	default:
		return false
	}
}

func builtInMetadataFromEntry(entry builtInBundleEntry) *model.PluginBuiltInMetadata {
	metadata := &model.PluginBuiltInMetadata{
		Official:            true,
		DocsRef:             entry.DocsRef,
		VerificationProfile: entry.VerificationProfile,
		CoreFlows:           append([]string(nil), entry.CoreFlows...),
		StarterFamily:       strings.TrimSpace(entry.StarterFamily),
		DependencyRefs:      append([]string(nil), entry.DependencyRefs...),
		WorkspaceRefs:       append([]string(nil), entry.WorkspaceRefs...),
		Installable:         true,
	}

	if entry.Readiness == nil {
		metadata.AvailabilityStatus = entry.Availability.Status
		metadata.AvailabilityMessage = entry.Availability.Message
		metadata.ReadinessStatus = entry.Availability.Status
		metadata.ReadinessMessage = entry.Availability.Message
		if entry.Availability.Status == "unavailable" {
			metadata.Installable = false
			metadata.InstallBlockedReason = entry.Availability.Message
		}
		return metadata
	}

	readiness := entry.Readiness
	if readiness.Installable != nil {
		metadata.Installable = *readiness.Installable
	}

	host := normalizeBuiltInHost(runtime.GOOS)
	if len(readiness.SupportedHosts) > 0 && !containsNormalizedHost(readiness.SupportedHosts, host) {
		return applyBuiltInReadinessState(metadata, "unsupported_host", firstNonEmptyBuiltIn(readiness.BlockedMessage, entry.Availability.Message, "Built-in plugin is unsupported on this host."), readiness.NextStep, nil, nil, []string{"unsupported_host"})
	}

	missingPrerequisites := collectMissingReadinessRequirements(readiness.Prerequisites, func(req builtInBundleRequirement) bool {
		switch strings.ToLower(strings.TrimSpace(req.Kind)) {
		case "executable":
			_, err := exec.LookPath(strings.TrimSpace(req.Value))
			return err == nil
		default:
			return true
		}
	})
	if len(missingPrerequisites) > 0 {
		return applyBuiltInReadinessState(metadata, "requires_prerequisite", firstNonEmptyBuiltIn(readiness.BlockedMessage, entry.Availability.Message, "Built-in plugin requires a local prerequisite before activation can succeed."), readiness.NextStep, missingPrerequisites, nil, []string{"missing_prerequisite"})
	}

	missingConfiguration := collectMissingReadinessRequirements(readiness.Configuration, func(req builtInBundleRequirement) bool {
		switch strings.ToLower(strings.TrimSpace(req.Kind)) {
		case "env":
			value, ok := os.LookupEnv(strings.TrimSpace(req.Value))
			return ok && strings.TrimSpace(value) != ""
		default:
			return true
		}
	})
	if len(missingConfiguration) > 0 {
		return applyBuiltInReadinessState(metadata, "requires_configuration", firstNonEmptyBuiltIn(readiness.BlockedMessage, entry.Availability.Message, "Built-in plugin requires configuration before activation can succeed."), readiness.NextStep, nil, missingConfiguration, []string{"missing_configuration"})
	}

	return applyBuiltInReadinessState(metadata, "ready", firstNonEmptyBuiltIn(readiness.ReadyMessage, entry.Availability.Message, "Built-in plugin is ready for install."), readiness.NextStep, nil, nil, nil)
}

func applyBuiltInReadinessState(metadata *model.PluginBuiltInMetadata, status string, message string, nextStep string, missingPrerequisites []string, missingConfiguration []string, blockingReasons []string) *model.PluginBuiltInMetadata {
	metadata.ReadinessStatus = status
	metadata.ReadinessMessage = message
	metadata.AvailabilityStatus = status
	metadata.AvailabilityMessage = message
	metadata.NextStep = nextStep
	metadata.MissingPrerequisites = append([]string(nil), missingPrerequisites...)
	metadata.MissingConfiguration = append([]string(nil), missingConfiguration...)
	metadata.BlockingReasons = append([]string(nil), blockingReasons...)
	if !metadata.Installable {
		metadata.InstallBlockedReason = firstNonEmptyBuiltIn(message, nextStep)
	}
	return metadata
}

func collectMissingReadinessRequirements(requirements []builtInBundleRequirement, predicate func(req builtInBundleRequirement) bool) []string {
	missing := make([]string, 0)
	for _, requirement := range requirements {
		if predicate(requirement) {
			continue
		}
		label := strings.TrimSpace(requirement.Label)
		if label == "" {
			label = strings.TrimSpace(requirement.Value)
		}
		if label == "" {
			continue
		}
		missing = append(missing, label)
	}
	return missing
}

func normalizeBuiltInHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}

func containsNormalizedHost(hosts []string, host string) bool {
	for _, candidate := range hosts {
		if normalizeBuiltInHost(candidate) == host {
			return true
		}
	}
	return false
}

func firstNonEmptyBuiltIn(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
