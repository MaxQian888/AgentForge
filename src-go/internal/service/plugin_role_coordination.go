package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
)

type pluginRoleListProvider interface {
	List() ([]*rolepkg.Manifest, error)
}

type pluginCatalogListProvider interface {
	List(ctx context.Context, filter PluginListFilter) ([]*model.PluginRecord, error)
}

type pluginBuiltInDiscoveryProvider interface {
	DiscoverBuiltIns(ctx context.Context) ([]*model.PluginRecord, error)
}

func BuildPluginRoleDependencies(record *model.PluginRecord, roleStore PluginRoleStore) []model.PluginRoleDependency {
	if record == nil || record.Kind != model.PluginKindWorkflow || record.Spec.Workflow == nil {
		return nil
	}

	referencesByRole := collectWorkflowRoleReferences(record.Spec.Workflow)
	roleIDs := make([]string, 0, len(referencesByRole))
	for roleID := range referencesByRole {
		roleIDs = append(roleIDs, roleID)
	}
	slices.Sort(roleIDs)

	dependencies := make([]model.PluginRoleDependency, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		dependency := model.PluginRoleDependency{
			RoleID:     roleID,
			Status:     "resolved",
			Blocking:   false,
			References: append([]string(nil), referencesByRole[roleID]...),
		}
		if roleStore == nil {
			dependency.Status = "missing"
			dependency.Blocking = true
			dependency.Message = fmt.Sprintf("Role %s cannot be validated because the role registry is not configured", roleID)
			dependencies = append(dependencies, dependency)
			continue
		}

		role, err := roleStore.Get(roleID)
		if err != nil {
			dependency.Status = "missing"
			dependency.Blocking = true
			dependency.Message = fmt.Sprintf("Role %s no longer resolves from the authoritative role registry", roleID)
			dependencies = append(dependencies, dependency)
			continue
		}
		dependency.RoleName = strings.TrimSpace(role.Metadata.Name)
		dependencies = append(dependencies, dependency)
	}

	return dependencies
}

func BuildPluginRoleConsumers(record *model.PluginRecord, roles []*rolepkg.Manifest) []model.PluginRoleConsumer {
	if record == nil || len(roles) == 0 {
		return nil
	}
	if record.Kind != model.PluginKindTool {
		return nil
	}

	status := pluginLifecycleDependencyStatus(record)
	consumers := make([]model.PluginRoleConsumer, 0)
	for _, role := range roles {
		if role == nil {
			continue
		}
		roleID := strings.TrimSpace(role.Metadata.ID)
		roleName := strings.TrimSpace(role.Metadata.Name)
		for _, dependency := range collectRolePluginReferences(role) {
			if dependency.PluginID != record.Metadata.ID {
				continue
			}
			consumer := model.PluginRoleConsumer{
				RoleID:        roleID,
				RoleName:      roleName,
				ReferenceType: dependency.ReferenceType,
				Status:        status,
				Blocking:      status != "active",
			}
			if consumer.Blocking {
				consumer.Message = fmt.Sprintf("Role %s depends on plugin %s but the plugin is currently %s", roleID, record.Metadata.ID, status)
			}
			consumers = append(consumers, consumer)
		}
	}

	slices.SortFunc(consumers, func(a, b model.PluginRoleConsumer) int {
		if compare := strings.Compare(a.RoleID, b.RoleID); compare != 0 {
			return compare
		}
		return strings.Compare(a.ReferenceType, b.ReferenceType)
	})
	return consumers
}

func BuildRolePluginDependencies(role *rolepkg.Manifest, plugins []*model.PluginRecord) []model.RolePluginDependency {
	if role == nil {
		return nil
	}
	index := make(map[string]*model.PluginRecord, len(plugins))
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		index[strings.TrimSpace(plugin.Metadata.ID)] = plugin
	}

	dependencies := collectRolePluginReferences(role)
	result := make([]model.RolePluginDependency, 0, len(dependencies))
	for _, dependency := range dependencies {
		summary := model.RolePluginDependency{
			PluginID:      dependency.PluginID,
			ReferenceType: dependency.ReferenceType,
			Status:        "missing",
			Blocking:      true,
			Message:       fmt.Sprintf("Role dependency %s is not currently installed as a usable tool plugin", dependency.PluginID),
		}

		plugin := index[dependency.PluginID]
		if plugin == nil {
			result = append(result, summary)
			continue
		}

		summary.PluginName = strings.TrimSpace(plugin.Metadata.Name)
		summary.PluginKind = string(plugin.Kind)
		summary.LifecycleState = string(plugin.LifecycleState)

		if plugin.Kind != model.PluginKindTool {
			summary.Status = "incompatible-kind"
			summary.Message = fmt.Sprintf("Role dependency %s resolves to %s instead of ToolPlugin", dependency.PluginID, plugin.Kind)
			result = append(result, summary)
			continue
		}

		if status, blocking, message, ok := builtInRoleDependencyState(plugin); ok {
			summary.Status = status
			summary.Blocking = blocking
			summary.Message = message
			result = append(result, summary)
			continue
		}

		if plugin.LifecycleState == model.PluginStateActive {
			summary.Status = "active"
			summary.Blocking = false
			summary.Message = ""
			result = append(result, summary)
			continue
		}

		summary.Status = pluginLifecycleDependencyStatus(plugin)
		summary.Message = fmt.Sprintf("Role dependency %s is installed but not currently active", dependency.PluginID)
		result = append(result, summary)
	}

	slices.SortFunc(result, func(a, b model.RolePluginDependency) int {
		if compare := strings.Compare(a.PluginID, b.PluginID); compare != 0 {
			return compare
		}
		return strings.Compare(a.ReferenceType, b.ReferenceType)
	})
	return result
}

func BuildRolePluginConsumers(roleID string, plugins []*model.PluginRecord) []model.RolePluginConsumer {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" || len(plugins) == 0 {
		return nil
	}

	consumers := make([]model.RolePluginConsumer, 0)
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		referencesByRole := collectWorkflowRoleReferences(plugin.Spec.Workflow)
		references, ok := referencesByRole[roleID]
		if !ok {
			continue
		}
		consumer := model.RolePluginConsumer{
			PluginID:       plugin.Metadata.ID,
			PluginName:     strings.TrimSpace(plugin.Metadata.Name),
			PluginKind:     string(plugin.Kind),
			LifecycleState: string(plugin.LifecycleState),
			Status:         string(plugin.LifecycleState),
			Blocking:       true,
			References:     append([]string(nil), references...),
			Message:        fmt.Sprintf("Plugin %s still binds role %s", plugin.Metadata.ID, roleID),
		}
		consumers = append(consumers, consumer)
	}

	slices.SortFunc(consumers, func(a, b model.RolePluginConsumer) int {
		return strings.Compare(a.PluginID, b.PluginID)
	})
	return consumers
}

func HasBlockingRolePluginDependencies(dependencies []model.RolePluginDependency) bool {
	for _, dependency := range dependencies {
		if dependency.Blocking {
			return true
		}
	}
	return false
}

func JoinBlockingRolePluginDependencyMessages(dependencies []model.RolePluginDependency) string {
	messages := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency.Blocking && strings.TrimSpace(dependency.Message) != "" {
			messages = append(messages, dependency.Message)
		}
	}
	return strings.Join(messages, "; ")
}

func ListDependencyPlugins(ctx context.Context, catalog pluginCatalogListProvider) ([]*model.PluginRecord, error) {
	if catalog == nil {
		return nil, nil
	}
	plugins, err := catalog.List(ctx, PluginListFilter{})
	if err != nil {
		return nil, err
	}

	discoverer, ok := catalog.(pluginBuiltInDiscoveryProvider)
	if !ok {
		return plugins, nil
	}
	builtIns, err := discoverer.DiscoverBuiltIns(ctx)
	if err != nil {
		return nil, err
	}
	if len(builtIns) == 0 {
		return plugins, nil
	}

	merged := make([]*model.PluginRecord, 0, len(plugins)+len(builtIns))
	indexByID := make(map[string]int, len(plugins)+len(builtIns))
	for _, plugin := range plugins {
		if plugin == nil {
			continue
		}
		merged = append(merged, plugin)
		indexByID[strings.TrimSpace(plugin.Metadata.ID)] = len(merged) - 1
	}
	for _, plugin := range builtIns {
		if plugin == nil {
			continue
		}
		pluginID := strings.TrimSpace(plugin.Metadata.ID)
		if existingIndex, ok := indexByID[pluginID]; ok {
			if merged[existingIndex].BuiltIn == nil && plugin.BuiltIn != nil {
				cloned := *merged[existingIndex]
				cloned.BuiltIn = cloneBuiltInMetadata(plugin.BuiltIn)
				merged[existingIndex] = &cloned
			}
			continue
		}
		merged = append(merged, plugin)
		indexByID[pluginID] = len(merged) - 1
	}
	return merged, nil
}

func ListDependencyRoles(store PluginRoleStore) ([]*rolepkg.Manifest, error) {
	if store == nil {
		return nil, nil
	}
	lister, ok := store.(pluginRoleListProvider)
	if !ok {
		return nil, nil
	}
	return lister.List()
}

func FirstMissingWorkflowRoleError(record *model.PluginRecord, roleStore PluginRoleStore) error {
	for _, dependency := range BuildPluginRoleDependencies(record, roleStore) {
		if dependency.Blocking {
			return fmt.Errorf("unknown workflow role reference: %s", dependency.RoleID)
		}
	}
	return nil
}

func collectWorkflowRoleReferences(workflow *model.WorkflowPluginSpec) map[string][]string {
	if workflow == nil {
		return nil
	}
	referencesByRole := make(map[string][]string)
	for _, binding := range workflow.Roles {
		roleID := strings.TrimSpace(binding.ID)
		if roleID == "" {
			continue
		}
		referencesByRole[roleID] = appendIfMissing(referencesByRole[roleID], "workflow.roles")
	}
	for _, step := range workflow.Steps {
		roleID := strings.TrimSpace(step.Role)
		if roleID == "" {
			continue
		}
		referencesByRole[roleID] = appendIfMissing(referencesByRole[roleID], fmt.Sprintf("steps.%s.role", step.ID))
	}
	return referencesByRole
}

type rolePluginReference struct {
	PluginID      string
	ReferenceType string
}

func collectRolePluginReferences(role *rolepkg.Manifest) []rolePluginReference {
	if role == nil {
		return nil
	}
	seen := make(map[string]struct{})
	references := make([]rolePluginReference, 0)
	for _, pluginID := range role.Capabilities.ToolConfig.External {
		normalized := strings.TrimSpace(pluginID)
		if normalized == "" {
			continue
		}
		key := "external:" + normalized
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		references = append(references, rolePluginReference{PluginID: normalized, ReferenceType: "external"})
	}
	for _, server := range role.Capabilities.ToolConfig.MCPServers {
		normalized := strings.TrimSpace(server.Name)
		if normalized == "" {
			continue
		}
		key := "mcp_server:" + normalized
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		references = append(references, rolePluginReference{PluginID: normalized, ReferenceType: "mcp_server"})
	}
	return references
}

func pluginLifecycleDependencyStatus(plugin *model.PluginRecord) string {
	if plugin == nil {
		return "missing"
	}
	switch plugin.LifecycleState {
	case model.PluginStateActive:
		return "active"
	case model.PluginStateActivating:
		return "activating"
	case model.PluginStateEnabled:
		return "enabled"
	case model.PluginStateInstalled:
		return "installed"
	case model.PluginStateDisabled:
		return "disabled"
	case model.PluginStateDegraded:
		return "degraded"
	default:
		return "missing"
	}
}

func IsMissingRoleError(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func builtInRoleDependencyState(plugin *model.PluginRecord) (string, bool, string, bool) {
	if plugin == nil || plugin.BuiltIn == nil || !plugin.BuiltIn.Official {
		return "", false, "", false
	}
	if plugin.LifecycleState == model.PluginStateActive {
		return "active", false, "", true
	}

	status := strings.TrimSpace(plugin.BuiltIn.ReadinessStatus)
	if status == "" {
		status = strings.TrimSpace(plugin.BuiltIn.AvailabilityStatus)
	}
	if status == "" || status == "ready" {
		return "active", false, "", true
	}

	message := firstNonEmptyDependencyMessage(
		plugin.BuiltIn.ReadinessMessage,
		plugin.BuiltIn.AvailabilityMessage,
		fmt.Sprintf("Role dependency %s is bundled with AgentForge but not ready on this host", plugin.Metadata.ID),
	)
	return status, true, message, true
}

func cloneBuiltInMetadata(metadata *model.PluginBuiltInMetadata) *model.PluginBuiltInMetadata {
	if metadata == nil {
		return nil
	}
	cloned := *metadata
	cloned.BlockingReasons = append([]string(nil), metadata.BlockingReasons...)
	cloned.MissingPrerequisites = append([]string(nil), metadata.MissingPrerequisites...)
	cloned.MissingConfiguration = append([]string(nil), metadata.MissingConfiguration...)
	return &cloned
}

func firstNonEmptyDependencyMessage(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func appendIfMissing(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
