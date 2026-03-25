// Package role provides YAML role manifest parsing and a preset registry.
package role

import "github.com/react-go-quick-starter/server/internal/model"

// Manifest is an alias for model.RoleManifest for convenience.
type Manifest = model.RoleManifest
type Metadata = model.RoleMetadata
type Identity = model.RoleIdentity
type Capabilities = model.RoleCapabilities
type Knowledge = model.RoleKnowledge
type Security = model.RoleSecurity
type RoleResponseStyle = model.RoleResponseStyle
type RoleToolConfig = model.RoleToolConfig
type RoleMCPServer = model.RoleMCPServer
type RoleSkillReference = model.RoleSkillReference
type RoleKnowledgeSource = model.RoleKnowledgeSource
type RoleMemoryConfig = model.RoleMemoryConfig
type RoleShortTermMemory = model.RoleShortTermMemory
type RoleEpisodicMemory = model.RoleEpisodicMemory
type RoleSemanticMemory = model.RoleSemanticMemory
type RoleProceduralMemory = model.RoleProceduralMemory
type RolePermissions = model.RolePermissions
type RoleFileAccessPermission = model.RoleFileAccessPermission
type RoleNetworkPermission = model.RoleNetworkPermission
type RoleCodeExecutionPermission = model.RoleCodeExecutionPermission
type RoleResourceLimits = model.RoleResourceLimits
type RoleTokenBudgetLimit = model.RoleTokenBudgetLimit
type RoleAPICallsLimit = model.RoleAPICallsLimit
type RoleExecutionTimeLimit = model.RoleExecutionTimeLimit
type RoleCostLimit = model.RoleCostLimit
type RoleCollaboration = model.RoleCollaboration
type RoleCommunicationPrefs = model.RoleCommunicationPrefs
type RoleTrigger = model.RoleTrigger
type ExecutionProfile = model.RoleExecutionProfile
