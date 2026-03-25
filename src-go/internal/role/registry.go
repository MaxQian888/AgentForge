package role

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

// Registry holds loaded role manifests indexed by name.
type Registry struct {
	mu    sync.RWMutex
	roles map[string]*Manifest
}

// NewRegistry creates an empty role registry.
func NewRegistry() *Registry {
	return &Registry{
		roles: make(map[string]*Manifest),
	}
}

// LoadDir loads all .yaml and .yml files from a directory into the registry.
func (r *Registry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read roles dir %s: %w", dir, err)
	}

	discovered := make(map[string]*Manifest)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, entry.Name(), "role.yaml")
		if _, err := os.Stat(path); err != nil {
			continue
		}

		manifest, err := ParseFile(path)
		if err != nil {
			log.WithError(err).WithField("path", path).Warn("skipping invalid canonical role file")
			continue
		}
		discovered[roleKey(manifest)] = manifest
		log.WithFields(log.Fields{"id": roleKey(manifest), "path": path}).Info("discovered canonical role")
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		manifest, err := ParseFile(path)
		if err != nil {
			log.WithError(err).WithField("path", path).Warn("skipping invalid role file")
			continue
		}

		key := roleKey(manifest)
		if _, exists := discovered[key]; exists {
			log.WithFields(log.Fields{"id": key, "path": path}).Info("skipping legacy role because canonical manifest exists")
			continue
		}
		discovered[key] = manifest
		log.WithFields(log.Fields{"id": key, "path": path}).Info("discovered legacy role")
	}

	resolved := make(map[string]*Manifest, len(discovered))
	for key := range discovered {
		manifest, err := resolveManifest(key, discovered, resolved, make(map[string]bool))
		if err != nil {
			log.WithError(err).WithField("id", key).Warn("skipping unresolved role")
			continue
		}
		resolved[key] = manifest
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.roles = resolved

	return nil
}

// Get returns a role manifest by name.
func (r *Registry) Get(name string) (*Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.roles[name]
	return m, ok
}

// List returns all loaded role names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.roles))
	for name := range r.roles {
		names = append(names, name)
	}
	return names
}

// All returns all loaded manifests.
func (r *Registry) All() map[string]*Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Manifest, len(r.roles))
	for k, v := range r.roles {
		result[k] = v
	}
	return result
}

// Register adds a role manifest to the registry.
func (r *Registry) Register(manifest *Manifest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roles[roleKey(manifest)] = cloneManifest(manifest)
}

// Count returns the number of loaded roles.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.roles)
}

func resolveManifest(key string, discovered map[string]*Manifest, resolved map[string]*Manifest, visiting map[string]bool) (*Manifest, error) {
	if manifest, ok := resolved[key]; ok {
		return cloneManifest(manifest), nil
	}
	if visiting[key] {
		return nil, fmt.Errorf("cyclic inheritance detected for role %s", key)
	}

	manifest, ok := discovered[key]
	if !ok {
		return nil, fmt.Errorf("role %s not found", key)
	}

	visiting[key] = true
	defer delete(visiting, key)

	if manifest.Extends == "" {
		return cloneManifest(manifest), nil
	}

	parentKey := strings.TrimSpace(manifest.Extends)
	parent, err := resolveManifest(parentKey, discovered, resolved, visiting)
	if err != nil {
		return nil, fmt.Errorf("resolve parent %s for role %s: %w", parentKey, key, err)
	}

	return mergeManifests(parent, manifest), nil
}

func mergeManifests(parent, child *Manifest) *Manifest {
	if parent == nil {
		return cloneManifest(child)
	}
	if child == nil {
		return cloneManifest(parent)
	}

	merged := cloneManifest(child)
	if merged.APIVersion == "" {
		merged.APIVersion = parent.APIVersion
	}
	if merged.Kind == "" {
		merged.Kind = parent.Kind
	}

	merged.Metadata = mergeMetadata(parent.Metadata, child.Metadata)
	merged.Identity = mergeIdentity(parent.Identity, child.Identity)
	merged.SystemPrompt = firstNonEmpty(child.SystemPrompt, parent.SystemPrompt)
	merged.Capabilities = mergeCapabilities(parent.Capabilities, child.Capabilities)
	merged.Knowledge = mergeKnowledge(parent.Knowledge, child.Knowledge)
	merged.Security = mergeSecurity(parent.Security, child.Security)
	merged.Collaboration = mergeCollaboration(parent.Collaboration, child.Collaboration)
	merged.Triggers = mergeTriggers(parent.Triggers, child.Triggers)

	return merged
}

func mergeMetadata(parent, child Metadata) Metadata {
	merged := parent
	if child.ID != "" {
		merged.ID = child.ID
	}
	if child.Name != "" {
		merged.Name = child.Name
	}
	if child.Version != "" {
		merged.Version = child.Version
	}
	if child.Description != "" {
		merged.Description = child.Description
	}
	if child.Author != "" {
		merged.Author = child.Author
	}
	if child.Icon != "" {
		merged.Icon = child.Icon
	}
	merged.Tags = mergeUniqueStrings(parent.Tags, child.Tags)
	return merged
}

func mergeIdentity(parent, child Identity) Identity {
	merged := parent
	if child.Role != "" {
		merged.Role = child.Role
	}
	if child.Goal != "" {
		merged.Goal = child.Goal
	}
	if child.Backstory != "" {
		merged.Backstory = child.Backstory
	}
	if child.SystemPrompt != "" {
		merged.SystemPrompt = child.SystemPrompt
	}
	if child.Persona != "" {
		merged.Persona = child.Persona
	}
	if len(child.Goals) > 0 {
		merged.Goals = append([]string(nil), child.Goals...)
	}
	if len(child.Constraints) > 0 {
		merged.Constraints = append([]string(nil), child.Constraints...)
	}
	if child.Personality != "" {
		merged.Personality = child.Personality
	}
	if child.Language != "" {
		merged.Language = child.Language
	}
	if child.ResponseStyle != (RoleResponseStyle{}) {
		merged.ResponseStyle = child.ResponseStyle
	}
	return merged
}

func mergeCapabilities(parent, child Capabilities) Capabilities {
	merged := parent
	merged.Packages = mergeUniqueStrings(parent.Packages, child.Packages)
	if len(child.AllowedTools) > 0 {
		merged.AllowedTools = append([]string(nil), child.AllowedTools...)
		merged.Tools = append([]string(nil), child.AllowedTools...)
	}
	if len(child.Tools) > 0 {
		merged.Tools = append([]string(nil), child.Tools...)
	}
	if !isEmptyToolConfig(child.ToolConfig) {
		merged.ToolConfig = child.ToolConfig
	}
	merged.Skills = mergeSkillReferences(parent.Skills, child.Skills)
	if len(child.Languages) > 0 {
		merged.Languages = mergeUniqueStrings(parent.Languages, child.Languages)
	}
	if len(child.Frameworks) > 0 {
		merged.Frameworks = mergeUniqueStrings(parent.Frameworks, child.Frameworks)
	}
	if child.MaxConcurrency > 0 {
		merged.MaxConcurrency = child.MaxConcurrency
	}
	if child.MaxTurns > 0 {
		merged.MaxTurns = child.MaxTurns
	}
	if child.MaxBudgetUsd > 0 {
		merged.MaxBudgetUsd = child.MaxBudgetUsd
	}
	if len(child.CustomSettings) > 0 {
		merged.CustomSettings = cloneStringMap(child.CustomSettings)
	}
	return merged
}

func mergeKnowledge(parent, child Knowledge) Knowledge {
	merged := parent
	merged.Repositories = mergeUniqueStrings(parent.Repositories, child.Repositories)
	merged.Documents = mergeUniqueStrings(parent.Documents, child.Documents)
	merged.Patterns = mergeUniqueStrings(parent.Patterns, child.Patterns)
	if child.SystemPrompt != "" {
		merged.SystemPrompt = child.SystemPrompt
	}
	merged.Shared = mergeKnowledgeSources(parent.Shared, child.Shared)
	merged.Private = mergeKnowledgeSources(parent.Private, child.Private)
	merged.Memory = mergeMemoryConfig(parent.Memory, child.Memory)
	return merged
}

func isEmptyToolConfig(config RoleToolConfig) bool {
	return len(config.BuiltIn) == 0 && len(config.External) == 0 && len(config.MCPServers) == 0
}

func mergeSecurity(parent, child Security) Security {
	merged := parent
	merged.PermissionMode = stricterPermissionMode(parent.PermissionMode, child.PermissionMode)
	merged.AllowedPaths = stricterAllowedPaths(parent.AllowedPaths, child.AllowedPaths)
	merged.DeniedPaths = mergeUniqueStrings(parent.DeniedPaths, child.DeniedPaths)
	merged.MaxBudgetUsd = smallerPositive(parent.MaxBudgetUsd, child.MaxBudgetUsd)
	merged.RequireReview = parent.RequireReview || child.RequireReview
	merged.Profile = stricterSecurityProfile(parent.Profile, child.Profile)
	merged.Permissions = mergePermissions(parent.Permissions, child.Permissions)
	merged.OutputFilters = mergeUniqueStrings(parent.OutputFilters, child.OutputFilters)
	merged.ResourceLimits = mergeResourceLimits(parent.ResourceLimits, child.ResourceLimits)
	return merged
}

func mergeCollaboration(parent, child RoleCollaboration) RoleCollaboration {
	merged := parent
	merged.CanDelegateTo = mergeUniqueStrings(parent.CanDelegateTo, child.CanDelegateTo)
	merged.AcceptsDelegationFrom = mergeUniqueStrings(parent.AcceptsDelegationFrom, child.AcceptsDelegationFrom)
	if child.Communication.PreferredChannel != "" {
		merged.Communication.PreferredChannel = child.Communication.PreferredChannel
	}
	if child.Communication.ReportFormat != "" {
		merged.Communication.ReportFormat = child.Communication.ReportFormat
	}
	if child.Communication.EscalationPolicy != "" {
		merged.Communication.EscalationPolicy = child.Communication.EscalationPolicy
	}
	return merged
}

func mergeTriggers(parent, child []RoleTrigger) []RoleTrigger {
	if len(parent) == 0 && len(child) == 0 {
		return nil
	}

	merged := make([]RoleTrigger, 0, len(parent)+len(child))
	indexByKey := make(map[string]int, len(parent)+len(child))
	for _, item := range parent {
		key := triggerKey(item)
		if key == "" {
			continue
		}
		indexByKey[key] = len(merged)
		merged = append(merged, item)
	}
	for _, item := range child {
		key := triggerKey(item)
		if key == "" {
			continue
		}
		if index, ok := indexByKey[key]; ok {
			merged[index] = item
			continue
		}
		indexByKey[key] = len(merged)
		merged = append(merged, item)
	}
	return merged
}

func stricterPermissionMode(parent, child string) string {
	switch {
	case parent == "":
		return child
	case child == "":
		return parent
	case parent == "default" || child == "default":
		return "default"
	default:
		return child
	}
}

func stricterAllowedPaths(parent, child []string) []string {
	switch {
	case len(parent) == 0:
		return append([]string(nil), child...)
	case len(child) == 0:
		return append([]string(nil), parent...)
	}

	set := make(map[string]struct{}, len(parent))
	for _, value := range parent {
		set[value] = struct{}{}
	}

	result := make([]string, 0, len(child))
	for _, value := range child {
		if _, ok := set[value]; ok {
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		if len(child) < len(parent) {
			return append([]string(nil), child...)
		}
		return append([]string(nil), parent...)
	}

	slices.Sort(result)
	return result
}

func smallerPositive(values ...float64) float64 {
	best := 0.0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if best == 0 || value < best {
			best = value
		}
	}
	return best
}

func smallerPositiveInt(values ...int) int {
	best := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if best == 0 || value < best {
			best = value
		}
	}
	return best
}

func mergeUniqueStrings(base, extra []string) []string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(base)+len(extra))
	result := make([]string, 0, len(base)+len(extra))
	for _, item := range append(append([]string(nil), base...), extra...) {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func mergeKnowledgeSources(base, extra []RoleKnowledgeSource) []RoleKnowledgeSource {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}

	merged := make([]RoleKnowledgeSource, 0, len(base)+len(extra))
	indexByKey := make(map[string]int, len(base)+len(extra))
	for _, item := range base {
		key := knowledgeSourceKey(item)
		if key == "" {
			continue
		}
		indexByKey[key] = len(merged)
		merged = append(merged, item)
	}
	for _, item := range extra {
		key := knowledgeSourceKey(item)
		if key == "" {
			continue
		}
		if index, ok := indexByKey[key]; ok {
			merged[index] = item
			continue
		}
		indexByKey[key] = len(merged)
		merged = append(merged, item)
	}
	return merged
}

func mergeMemoryConfig(parent, child RoleMemoryConfig) RoleMemoryConfig {
	merged := parent
	if child.ShortTerm.MaxTokens > 0 {
		merged.ShortTerm.MaxTokens = child.ShortTerm.MaxTokens
	}
	if child.Episodic.Enabled {
		merged.Episodic.Enabled = true
	}
	if child.Episodic.RetentionDays > 0 {
		merged.Episodic.RetentionDays = child.Episodic.RetentionDays
	}
	if child.Semantic.Enabled {
		merged.Semantic.Enabled = true
	}
	if child.Semantic.AutoExtract {
		merged.Semantic.AutoExtract = true
	}
	if child.Procedural.Enabled {
		merged.Procedural.Enabled = true
	}
	if child.Procedural.LearnFromFeedback {
		merged.Procedural.LearnFromFeedback = true
	}
	return merged
}

func mergePermissions(parent, child RolePermissions) RolePermissions {
	merged := parent
	merged.FileAccess.AllowedPaths = stricterAllowedPaths(parent.FileAccess.AllowedPaths, child.FileAccess.AllowedPaths)
	merged.FileAccess.DeniedPaths = mergeUniqueStrings(parent.FileAccess.DeniedPaths, child.FileAccess.DeniedPaths)
	merged.Network.AllowedDomains = stricterAllowedPaths(parent.Network.AllowedDomains, child.Network.AllowedDomains)
	merged.CodeExecution.Sandbox = parent.CodeExecution.Sandbox || child.CodeExecution.Sandbox
	merged.CodeExecution.AllowedLanguages = stricterAllowedPaths(parent.CodeExecution.AllowedLanguages, child.CodeExecution.AllowedLanguages)
	return merged
}

func mergeResourceLimits(parent, child RoleResourceLimits) RoleResourceLimits {
	merged := parent
	merged.TokenBudget.PerTask = smallerPositiveInt(parent.TokenBudget.PerTask, child.TokenBudget.PerTask)
	merged.TokenBudget.PerDay = smallerPositiveInt(parent.TokenBudget.PerDay, child.TokenBudget.PerDay)
	merged.TokenBudget.PerMonth = smallerPositiveInt(parent.TokenBudget.PerMonth, child.TokenBudget.PerMonth)
	merged.APICalls.PerMinute = smallerPositiveInt(parent.APICalls.PerMinute, child.APICalls.PerMinute)
	merged.APICalls.PerHour = smallerPositiveInt(parent.APICalls.PerHour, child.APICalls.PerHour)
	if child.ExecutionTime.PerTask != "" {
		merged.ExecutionTime.PerTask = child.ExecutionTime.PerTask
	}
	if child.ExecutionTime.PerDay != "" {
		merged.ExecutionTime.PerDay = child.ExecutionTime.PerDay
	}
	if child.CostLimit.PerTask != "" {
		merged.CostLimit.PerTask = child.CostLimit.PerTask
	}
	if child.CostLimit.PerDay != "" {
		merged.CostLimit.PerDay = child.CostLimit.PerDay
	}
	if child.CostLimit.AlertThreshold > 0 {
		merged.CostLimit.AlertThreshold = child.CostLimit.AlertThreshold
	}
	return merged
}

func stricterSecurityProfile(parent, child string) string {
	order := map[string]int{
		"development":   1,
		"standard":      2,
		"high_security": 3,
	}
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	if order[parent] >= order[child] {
		return parent
	}
	return child
}

func knowledgeSourceKey(source RoleKnowledgeSource) string {
	switch {
	case source.ID != "":
		return source.ID
	case source.Description != "":
		return source.Type + "|" + source.Access + "|" + source.Description
	case len(source.Sources) > 0:
		return source.Type + "|" + source.Access + "|" + strings.Join(source.Sources, ",")
	default:
		return source.Type + "|" + source.Access
	}
}

func triggerKey(trigger RoleTrigger) string {
	if trigger.Event == "" && trigger.Action == "" && trigger.Condition == "" {
		return ""
	}
	return trigger.Event + "|" + trigger.Action + "|" + trigger.Condition
}

func roleKey(manifest *Manifest) string {
	if manifest == nil {
		return ""
	}
	if manifest.Metadata.ID != "" {
		return manifest.Metadata.ID
	}
	return manifest.Metadata.Name
}

func cloneManifest(manifest *Manifest) *Manifest {
	if manifest == nil {
		return nil
	}
	cloned := *manifest
	cloned.Metadata.Tags = append([]string(nil), manifest.Metadata.Tags...)
	cloned.Identity.Goals = append([]string(nil), manifest.Identity.Goals...)
	cloned.Identity.Constraints = append([]string(nil), manifest.Identity.Constraints...)
	cloned.Capabilities.Packages = append([]string(nil), manifest.Capabilities.Packages...)
	cloned.Capabilities.AllowedTools = append([]string(nil), manifest.Capabilities.AllowedTools...)
	cloned.Capabilities.Tools = append([]string(nil), manifest.Capabilities.Tools...)
	cloned.Capabilities.ToolConfig.BuiltIn = append([]string(nil), manifest.Capabilities.ToolConfig.BuiltIn...)
	cloned.Capabilities.ToolConfig.External = append([]string(nil), manifest.Capabilities.ToolConfig.External...)
	cloned.Capabilities.ToolConfig.MCPServers = append([]RoleMCPServer(nil), manifest.Capabilities.ToolConfig.MCPServers...)
	cloned.Capabilities.Skills = append([]RoleSkillReference(nil), manifest.Capabilities.Skills...)
	cloned.Capabilities.Languages = append([]string(nil), manifest.Capabilities.Languages...)
	cloned.Capabilities.Frameworks = append([]string(nil), manifest.Capabilities.Frameworks...)
	cloned.Capabilities.CustomSettings = cloneStringMap(manifest.Capabilities.CustomSettings)
	cloned.Knowledge.Repositories = append([]string(nil), manifest.Knowledge.Repositories...)
	cloned.Knowledge.Documents = append([]string(nil), manifest.Knowledge.Documents...)
	cloned.Knowledge.Patterns = append([]string(nil), manifest.Knowledge.Patterns...)
	cloned.Knowledge.Shared = append([]RoleKnowledgeSource(nil), manifest.Knowledge.Shared...)
	cloned.Knowledge.Private = append([]RoleKnowledgeSource(nil), manifest.Knowledge.Private...)
	for index := range cloned.Knowledge.Shared {
		cloned.Knowledge.Shared[index].Sources = append([]string(nil), manifest.Knowledge.Shared[index].Sources...)
	}
	for index := range cloned.Knowledge.Private {
		cloned.Knowledge.Private[index].Sources = append([]string(nil), manifest.Knowledge.Private[index].Sources...)
	}
	cloned.Security.AllowedPaths = append([]string(nil), manifest.Security.AllowedPaths...)
	cloned.Security.DeniedPaths = append([]string(nil), manifest.Security.DeniedPaths...)
	cloned.Security.Permissions.FileAccess.AllowedPaths = append([]string(nil), manifest.Security.Permissions.FileAccess.AllowedPaths...)
	cloned.Security.Permissions.FileAccess.DeniedPaths = append([]string(nil), manifest.Security.Permissions.FileAccess.DeniedPaths...)
	cloned.Security.Permissions.Network.AllowedDomains = append([]string(nil), manifest.Security.Permissions.Network.AllowedDomains...)
	cloned.Security.Permissions.CodeExecution.AllowedLanguages = append([]string(nil), manifest.Security.Permissions.CodeExecution.AllowedLanguages...)
	cloned.Security.OutputFilters = append([]string(nil), manifest.Security.OutputFilters...)
	if manifest.Overrides != nil {
		cloned.Overrides = make(map[string]any, len(manifest.Overrides))
		for key, value := range manifest.Overrides {
			cloned.Overrides[key] = value
		}
	}
	if manifest.Triggers != nil {
		cloned.Triggers = append([]RoleTrigger(nil), manifest.Triggers...)
	}
	cloned.Collaboration.CanDelegateTo = append([]string(nil), manifest.Collaboration.CanDelegateTo...)
	cloned.Collaboration.AcceptsDelegationFrom = append([]string(nil), manifest.Collaboration.AcceptsDelegationFrom...)
	return &cloned
}

func mergeSkillReferences(parent, child []RoleSkillReference) []RoleSkillReference {
	if len(parent) == 0 && len(child) == 0 {
		return nil
	}

	merged := make([]RoleSkillReference, 0, len(parent)+len(child))
	indexByPath := make(map[string]int, len(parent)+len(child))

	for _, item := range parent {
		if item.Path == "" {
			continue
		}
		indexByPath[item.Path] = len(merged)
		merged = append(merged, RoleSkillReference{
			Path:     item.Path,
			AutoLoad: item.AutoLoad,
		})
	}

	for _, item := range child {
		if item.Path == "" {
			continue
		}

		next := RoleSkillReference{
			Path:     item.Path,
			AutoLoad: item.AutoLoad,
		}
		if index, ok := indexByPath[item.Path]; ok {
			merged[index] = next
			continue
		}

		indexByPath[item.Path] = len(merged)
		merged = append(merged, next)
	}

	return merged
}
