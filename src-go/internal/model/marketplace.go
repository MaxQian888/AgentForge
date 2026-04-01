package model

import "time"

type MarketplaceItemType string

const (
	MarketplaceItemTypePlugin MarketplaceItemType = "plugin"
	MarketplaceItemTypeSkill  MarketplaceItemType = "skill"
	MarketplaceItemTypeRole   MarketplaceItemType = "role"
)

type MarketplaceConsumerSurface string

const (
	MarketplaceConsumerSurfacePluginManagementPanel MarketplaceConsumerSurface = "plugin-management-panel"
	MarketplaceConsumerSurfaceRoleWorkspace         MarketplaceConsumerSurface = "roles-workspace"
	MarketplaceConsumerSurfaceRoleSkillCatalog      MarketplaceConsumerSurface = "role-skill-catalog"
)

type MarketplaceConsumptionStatus string

const (
	MarketplaceConsumptionStatusInstalled MarketplaceConsumptionStatus = "installed"
	MarketplaceConsumptionStatusBlocked   MarketplaceConsumptionStatus = "blocked"
	MarketplaceConsumptionStatusWarning   MarketplaceConsumptionStatus = "warning"
)

type MarketplaceErrorCode string

const (
	MarketplaceErrorUnconfigured       MarketplaceErrorCode = "marketplace_unconfigured"
	MarketplaceErrorUnavailable        MarketplaceErrorCode = "marketplace_unavailable"
	MarketplaceErrorInvalidResponse    MarketplaceErrorCode = "marketplace_invalid_response"
	MarketplaceErrorInvalidArtifact    MarketplaceErrorCode = "marketplace_invalid_artifact"
	MarketplaceErrorDownloadFailed     MarketplaceErrorCode = "marketplace_download_failed"
	MarketplaceErrorDigestMismatch     MarketplaceErrorCode = "marketplace_digest_mismatch"
	MarketplaceErrorInstallFailed      MarketplaceErrorCode = "marketplace_install_failed"
	MarketplaceErrorInstallUnsupported MarketplaceErrorCode = "marketplace_install_not_supported"
)

type MarketplaceConsumptionProvenance struct {
	SourceType        string `json:"sourceType,omitempty"`
	MarketplaceItemID string `json:"marketplaceItemId"`
	SelectedVersion   string `json:"selectedVersion,omitempty"`
	RecordID          string `json:"recordId,omitempty"`
	LocalPath         string `json:"localPath,omitempty"`
}

type MarketplaceConsumptionRecord struct {
	ItemID          string                            `json:"itemId"`
	ItemType        MarketplaceItemType               `json:"itemType"`
	Version         string                            `json:"version,omitempty"`
	Status          MarketplaceConsumptionStatus      `json:"status"`
	ConsumerSurface MarketplaceConsumerSurface        `json:"consumerSurface"`
	Installed       bool                              `json:"installed"`
	Used            bool                              `json:"used"`
	RecordID        string                            `json:"recordId,omitempty"`
	LocalPath       string                            `json:"localPath,omitempty"`
	Provenance      *MarketplaceConsumptionProvenance `json:"provenance,omitempty"`
	Warning         string                            `json:"warning,omitempty"`
	FailureReason   string                            `json:"failureReason,omitempty"`
	UpdatedAt       time.Time                         `json:"updatedAt,omitempty"`
}

type MarketplaceConsumptionResponse struct {
	Items []MarketplaceConsumptionRecord `json:"items"`
}

type MarketplaceInstallResponse struct {
	OK        bool                         `json:"ok"`
	Item      MarketplaceConsumptionRecord `json:"item"`
	ErrorCode MarketplaceErrorCode         `json:"errorCode,omitempty"`
	Message   string                       `json:"message,omitempty"`
}
