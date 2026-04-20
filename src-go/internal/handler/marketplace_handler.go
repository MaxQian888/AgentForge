package handler

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	rolepkg "github.com/agentforge/server/internal/role"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// MarketplaceWorkflowTemplateRepo is the repo interface needed for workflow template install.
type MarketplaceWorkflowTemplateRepo interface {
	Create(ctx context.Context, def *model.WorkflowDefinition) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
	Update(ctx context.Context, id uuid.UUID, def *model.WorkflowDefinition) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListTemplates(ctx context.Context, category string) ([]*model.WorkflowDefinition, error)
	ListTemplatesForProject(ctx context.Context, projectID uuid.UUID, query string, category string, source string) ([]*model.WorkflowDefinition, error)
	ListTemplatesByName(ctx context.Context, name string) ([]*model.WorkflowDefinition, error)
}

// MarketplaceProjectTemplateInstaller is the narrow contract the marketplace
// install seam needs to persist a project_template. Main backend wires in
// service.ProjectTemplateService via MaterializeMarketplaceInstall.
type MarketplaceProjectTemplateInstaller interface {
	MaterializeMarketplaceInstall(
		ctx context.Context,
		installer uuid.UUID,
		name, description, snapshotJSON string,
		snapshotVersion int,
	) (*model.ProjectTemplate, error)
}

// MarketplaceHandler handles marketplace install integration endpoints on the src-go backend.
type MarketplaceHandler struct {
	pluginSvc            *service.PluginService
	marketURL            string
	httpClient           *http.Client
	pluginsDir           string
	rolesDir             string
	imBridgePluginDir    string
	workflowTemplateRepo MarketplaceWorkflowTemplateRepo
	projectTemplateInst  MarketplaceProjectTemplateInstaller
}

// WithWorkflowTemplateRepo wires the workflow template repository for marketplace installs.
func (h *MarketplaceHandler) WithWorkflowTemplateRepo(repo MarketplaceWorkflowTemplateRepo) *MarketplaceHandler {
	h.workflowTemplateRepo = repo
	return h
}

// WithProjectTemplateInstaller wires the main backend's project template
// service so this handler can materialize marketplace-sourced project
// templates into project_templates (source=marketplace).
func (h *MarketplaceHandler) WithProjectTemplateInstaller(inst MarketplaceProjectTemplateInstaller) *MarketplaceHandler {
	h.projectTemplateInst = inst
	return h
}

// WithIMBridgePluginDir sets the directory shared with the IM Bridge for
// delivering marketplace-sourced command plugins. When empty, marketplace
// installs do not ship `im_commands` payloads to the bridge.
func (h *MarketplaceHandler) WithIMBridgePluginDir(dir string) *MarketplaceHandler {
	h.imBridgePluginDir = strings.TrimSpace(dir)
	return h
}

const marketplaceConsumptionStateFile = "marketplace-consumption.json"

// NewMarketplaceHandler creates a new MarketplaceHandler.
func NewMarketplaceHandler(pluginSvc *service.PluginService, marketURL string, pluginsDir string, rolesDir string) *MarketplaceHandler {
	return &MarketplaceHandler{
		pluginSvc:  pluginSvc,
		marketURL:  marketURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		pluginsDir: pluginsDir,
		rolesDir:   rolesDir,
	}
}

// MarketplaceInstallRequest is the request body for POST /marketplace/install.
type MarketplaceInstallRequest struct {
	ItemID  string `json:"item_id" validate:"required"`
	Version string `json:"version" validate:"required"`
}

// Install downloads an artifact from the marketplace service, verifies its digest,
// saves it locally, and registers it in the plugin service when the item type is "plugin".
func (h *MarketplaceHandler) Install(c echo.Context) error {
	req := new(MarketplaceInstallRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request"})
	}
	if req.ItemID == "" || req.Version == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "item_id and version are required"})
	}

	authHeader := c.Request().Header.Get("Authorization")
	ctx := c.Request().Context()
	// Stash the installer's user id on ctx so sub-install paths
	// (e.g. project_template) can materialize their rows against the right
	// owner without expanding every helper's signature.
	if uid, err := claimsUserID(c); err == nil && uid != nil {
		ctx = withMarketplaceInstallerUserID(ctx, *uid)
	}

	if strings.TrimSpace(h.marketURL) == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{
			"errorCode": model.MarketplaceErrorUnconfigured,
			"message":   "Marketplace service URL is not configured.",
		})
	}

	itemMeta, err := h.fetchItemMetadata(ctx, authHeader, req.ItemID)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]any{
			"errorCode": model.MarketplaceErrorInvalidResponse,
			"message":   err.Error(),
		})
	}

	installState := model.MarketplaceConsumptionRecord{
		ItemID:          req.ItemID,
		ItemType:        marketplaceItemType(itemMeta.Type),
		Version:         req.Version,
		Status:          model.MarketplaceConsumptionStatusBlocked,
		ConsumerSurface: marketplaceConsumerSurface(itemMeta.Type),
		Installed:       false,
		Used:            false,
		UpdatedAt:       time.Now().UTC(),
		Provenance: &model.MarketplaceConsumptionProvenance{
			SourceType:        string(model.PluginSourceMarketplace),
			MarketplaceItemID: req.ItemID,
			SelectedVersion:   req.Version,
		},
	}

	destDir := filepath.Join(h.marketplaceRootDir(), req.ItemID, req.Version)
	installState.LocalPath = destDir
	installState.Provenance.LocalPath = destDir

	// 1. Download artifact from marketplace service.
	downloadURL := fmt.Sprintf("%s/api/v1/items/%s/versions/%s/download",
		h.marketURL, req.ItemID, req.Version)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create request"})
	}
	httpReq.Header.Set("Authorization", authHeader)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		installState.FailureReason = "failed to reach marketplace service"
		_ = h.writeConsumptionState(installState)
		return c.JSON(http.StatusBadGateway, model.MarketplaceInstallResponse{
			OK:        false,
			Item:      installState,
			ErrorCode: model.MarketplaceErrorUnavailable,
			Message:   installState.FailureReason,
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		installState.FailureReason = "marketplace service returned error"
		_ = h.writeConsumptionState(installState)
		return c.JSON(http.StatusBadGateway, model.MarketplaceInstallResponse{
			OK:        false,
			Item:      installState,
			ErrorCode: model.MarketplaceErrorUnavailable,
			Message:   installState.FailureReason,
		})
	}

	expectedDigest := resp.Header.Get("X-Content-Digest")

	// 2. Save artifact while computing SHA-256.
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create directory"})
	}
	destPath := filepath.Join(destDir, "artifact")
	tmpPath := destPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create file"})
	}

	hasher := sha256.New()
	tee := io.TeeReader(resp.Body, hasher)
	if _, err := io.Copy(f, tee); err != nil {
		f.Close()
		os.Remove(tmpPath)
		installState.FailureReason = "failed to download artifact"
		_ = h.writeConsumptionState(installState)
		return c.JSON(http.StatusInternalServerError, model.MarketplaceInstallResponse{
			OK:        false,
			Item:      installState,
			ErrorCode: model.MarketplaceErrorDownloadFailed,
			Message:   installState.FailureReason,
		})
	}
	f.Close()

	// 3. Verify digest.
	actualDigest := hex.EncodeToString(hasher.Sum(nil))
	if expectedDigest != "" && actualDigest != expectedDigest {
		os.Remove(tmpPath)
		installState.FailureReason = "artifact digest mismatch"
		_ = h.writeConsumptionState(installState)
		return c.JSON(http.StatusBadRequest, model.MarketplaceInstallResponse{
			OK:        false,
			Item:      installState,
			ErrorCode: model.MarketplaceErrorDigestMismatch,
			Message:   installState.FailureReason,
		})
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to save artifact"})
	}

	if err := h.completeInstall(ctx, itemMeta, req, destDir, &installState); err != nil {
		if invalidArtifactErr, ok := err.(*marketplaceArtifactError); ok {
			installState.Status = model.MarketplaceConsumptionStatusBlocked
			installState.FailureReason = invalidArtifactErr.Error()
			if persistErr := h.writeConsumptionState(installState); persistErr != nil {
				return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to persist marketplace state"})
			}
			return c.JSON(http.StatusBadRequest, model.MarketplaceInstallResponse{
				OK:        false,
				Item:      installState,
				ErrorCode: model.MarketplaceErrorInvalidArtifact,
				Message:   invalidArtifactErr.Error(),
			})
		}

		installState.Status = model.MarketplaceConsumptionStatusWarning
		installState.Warning = err.Error()
		installState.FailureReason = err.Error()
		if persistErr := h.writeConsumptionState(installState); persistErr != nil {
			return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to persist marketplace state"})
		}
		return c.JSON(http.StatusConflict, model.MarketplaceInstallResponse{
			OK:        false,
			Item:      installState,
			ErrorCode: model.MarketplaceErrorInstallFailed,
			Message:   installState.Warning,
		})
	}

	if err := h.writeConsumptionState(installState); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to persist marketplace state"})
	}

	return c.JSON(http.StatusOK, model.MarketplaceInstallResponse{
		OK:   true,
		Item: installState,
	})
}

// Installed returns the list of marketplace catalog IDs that have been installed locally.
func (h *MarketplaceHandler) Installed(c echo.Context) error {
	plugins, err := h.pluginSvc.List(c.Request().Context(), service.PluginListFilter{
		SourceType: model.PluginSourceMarketplace,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list plugins"})
	}

	installedIDs := make([]string, 0, len(plugins))
	for _, p := range plugins {
		if p.Source.Catalog != "" {
			installedIDs = append(installedIDs, p.Source.Catalog)
		} else {
			installedIDs = append(installedIDs, p.Metadata.ID)
		}
	}

	return c.JSON(http.StatusOK, installedIDs)
}

// Consumption returns the typed marketplace install and consumption state known locally.
func (h *MarketplaceHandler) Consumption(c echo.Context) error {
	records, err := h.loadConsumptionStates()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to load marketplace state"})
	}

	byKey := make(map[string]model.MarketplaceConsumptionRecord, len(records))
	for _, record := range records {
		byKey[marketplaceConsumptionKey(record.ItemID, record.Version)] = record
	}

	plugins, err := h.pluginSvc.List(c.Request().Context(), service.PluginListFilter{
		SourceType: model.PluginSourceMarketplace,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list marketplace plugins"})
	}
	for _, plugin := range plugins {
		itemID := plugin.Source.Catalog
		if strings.TrimSpace(itemID) == "" {
			itemID = plugin.Metadata.ID
		}
		version := plugin.Source.Ref
		if strings.TrimSpace(version) == "" {
			version = plugin.Metadata.Version
		}
		record := model.MarketplaceConsumptionRecord{
			ItemID:          itemID,
			ItemType:        model.MarketplaceItemTypePlugin,
			Version:         version,
			Status:          model.MarketplaceConsumptionStatusInstalled,
			ConsumerSurface: model.MarketplaceConsumerSurfacePluginManagementPanel,
			Installed:       true,
			Used:            true,
			RecordID:        plugin.Metadata.ID,
			LocalPath:       plugin.Source.Path,
			UpdatedAt:       time.Now().UTC(),
			Provenance: &model.MarketplaceConsumptionProvenance{
				SourceType:        string(plugin.Source.Type),
				MarketplaceItemID: itemID,
				SelectedVersion:   version,
				RecordID:          plugin.Metadata.ID,
				LocalPath:         plugin.Source.Path,
			},
		}
		byKey[marketplaceConsumptionKey(itemID, version)] = record
	}

	builtInSkills, err := h.listBuiltInSkills()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list built-in skills"})
	}
	for _, skill := range builtInSkills {
		record := model.MarketplaceConsumptionRecord{
			ItemID:          skill.ID,
			ItemType:        model.MarketplaceItemTypeSkill,
			Status:          model.MarketplaceConsumptionStatusInstalled,
			ConsumerSurface: model.MarketplaceConsumerSurfaceRoleSkillCatalog,
			Installed:       true,
			Used:            false,
			RecordID:        skill.SkillPreview.CanonicalPath,
			LocalPath:       skill.LocalPath,
			UpdatedAt:       time.Now().UTC(),
			Provenance: &model.MarketplaceConsumptionProvenance{
				SourceType:        string(model.PluginSourceBuiltin),
				MarketplaceItemID: skill.ID,
				RecordID:          skill.SkillPreview.CanonicalPath,
				LocalPath:         skill.LocalPath,
			},
		}
		byKey[marketplaceConsumptionKey(skill.ID, "")] = record
	}

	items := make([]model.MarketplaceConsumptionRecord, 0, len(byKey))
	for _, item := range byKey {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ItemID == items[j].ItemID {
			return items[i].Version < items[j].Version
		}
		return items[i].ItemID < items[j].ItemID
	})

	return c.JSON(http.StatusOK, model.MarketplaceConsumptionResponse{Items: items})
}

func (h *MarketplaceHandler) ListBuiltInSkills(c echo.Context) error {
	items, err := h.listBuiltInSkills()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list built-in skills"})
	}
	return c.JSON(http.StatusOK, items)
}

func (h *MarketplaceHandler) GetBuiltInSkill(c echo.Context) error {
	requestedID := strings.TrimSpace(c.Param("id"))
	if requestedID == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "missing built-in skill id"})
	}

	items, err := h.listBuiltInSkills()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to load built-in skill"})
	}
	for _, item := range items {
		if item.ID == requestedID || item.Slug == requestedID {
			return c.JSON(http.StatusOK, item)
		}
	}
	return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "built-in skill not found"})
}

type marketplaceItemMetadata struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Name          string          `json:"name"`
	Slug          string          `json:"slug"`
	LatestVersion string          `json:"latest_version"`
	ExtraMetadata json.RawMessage `json:"extra_metadata,omitempty"`
}

// pluginIMCommandSpec mirrors the marketplace-side typed shape so the
// backend can parse `extra_metadata.im_commands` without depending on the
// marketplace module.
type pluginIMCommandSpec struct {
	Slash       string               `yaml:"slash" json:"slash"`
	Description string               `yaml:"description,omitempty" json:"description,omitempty"`
	ActionClass string               `yaml:"action_class,omitempty" json:"actionClass,omitempty"`
	Subcommands []pluginIMSubcommand `yaml:"subcommands,omitempty" json:"subcommands,omitempty"`
	Invoke      *pluginIMInvokeSpec  `yaml:"invoke,omitempty" json:"invoke,omitempty"`
}

type pluginIMSubcommand struct {
	Name        string              `yaml:"name" json:"name"`
	Description string              `yaml:"description,omitempty" json:"description,omitempty"`
	ActionClass string              `yaml:"action_class,omitempty" json:"actionClass,omitempty"`
	Invoke      *pluginIMInvokeSpec `yaml:"invoke" json:"invoke"`
}

type pluginIMInvokeSpec struct {
	Kind     string            `yaml:"kind" json:"kind"`
	URL      string            `yaml:"url,omitempty" json:"url,omitempty"`
	Method   string            `yaml:"method,omitempty" json:"method,omitempty"`
	Timeout  string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	ServerID string            `yaml:"serverId,omitempty" json:"serverId,omitempty"`
	Tool     string            `yaml:"tool,omitempty" json:"tool,omitempty"`
	Key      string            `yaml:"key,omitempty" json:"key,omitempty"`
}

type pluginExtraMetadata struct {
	ImCommands []pluginIMCommandSpec `json:"im_commands,omitempty"`
	Tenants    []string              `json:"im_tenants,omitempty"`
}

type marketplaceArtifactError struct {
	message string
}

func (e *marketplaceArtifactError) Error() string {
	return e.message
}

func (h *MarketplaceHandler) fetchItemMetadata(ctx context.Context, authHeader string, itemID string) (*marketplaceItemMetadata, error) {
	metaURL := fmt.Sprintf("%s/api/v1/items/%s", h.marketURL, itemID)
	metaReq, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata request: %w", err)
	}
	metaReq.Header.Set("Authorization", authHeader)

	metaResp, err := h.httpClient.Do(metaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch item metadata: %w", err)
	}
	defer metaResp.Body.Close()
	if metaResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("marketplace metadata returned status %d", metaResp.StatusCode)
	}

	var itemMeta marketplaceItemMetadata
	if err := json.NewDecoder(metaResp.Body).Decode(&itemMeta); err != nil {
		return nil, fmt.Errorf("failed to parse item metadata: %w", err)
	}
	return &itemMeta, nil
}

func (h *MarketplaceHandler) marketplaceRootDir() string {
	return filepath.Join(h.pluginsDir, "marketplace")
}

func (h *MarketplaceHandler) skillsRootDir() string {
	if strings.TrimSpace(h.rolesDir) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(h.rolesDir), "skills")
}

func (h *MarketplaceHandler) completeInstall(
	ctx context.Context,
	itemMeta *marketplaceItemMetadata,
	req *MarketplaceInstallRequest,
	destDir string,
	installState *model.MarketplaceConsumptionRecord,
) error {
	switch installState.ItemType {
	case model.MarketplaceItemTypeRole:
		return h.installRoleArtifact(itemMeta, destDir, installState)
	case model.MarketplaceItemTypeSkill:
		return h.installSkillArtifact(itemMeta, destDir, installState)
	case model.MarketplaceItemTypeWorkflowTemplate:
		return h.installWorkflowTemplateArtifact(ctx, itemMeta, destDir, installState)
	case model.MarketplaceItemTypeProjectTemplate:
		return h.installProjectTemplateArtifact(ctx, itemMeta, destDir, installState)
	default:
		stagingDir, err := h.extractZipPackage(filepath.Join(destDir, "artifact"), destDir)
		if err != nil {
			return err
		}
		targetDir := filepath.Join(destDir, "content")
		if err := replaceDirectory(targetDir, stagingDir); err != nil {
			return err
		}
		source := &model.PluginSource{
			Type:    model.PluginSourceMarketplace,
			Catalog: req.ItemID,
			Ref:     req.Version,
			Path:    targetDir,
		}
		record, err := h.pluginSvc.Install(ctx, service.PluginInstallRequest{
			Path:   targetDir,
			Source: source,
		})
		if err != nil {
			return err
		}
		installState.Status = model.MarketplaceConsumptionStatusInstalled
		installState.Installed = true
		installState.Used = true
		installState.RecordID = record.Metadata.ID
		installState.LocalPath = targetDir
		installState.Provenance.RecordID = record.Metadata.ID
		installState.Provenance.LocalPath = targetDir

		// Marketplace-sourced IM command manifests: when the item declares
		// `im_commands` in ExtraMetadata, materialize a plugin.yaml into
		// IM_BRIDGE_PLUGIN_DIR so the bridge's hot-reload watcher picks it
		// up. Missing dir or missing im_commands → no-op.
		if err := h.shipIMCommandsToBridge(itemMeta, req.ItemID); err != nil {
			log.Warnf("failed to ship marketplace im_commands to bridge: %v", err)
		}
		return nil
	}
}

// shipIMCommandsToBridge writes <pluginDir>/<itemID>/plugin.yaml when the
// marketplace item's ExtraMetadata contains an im_commands list.
func (h *MarketplaceHandler) shipIMCommandsToBridge(itemMeta *marketplaceItemMetadata, pluginID string) error {
	if h == nil || h.imBridgePluginDir == "" || itemMeta == nil || len(itemMeta.ExtraMetadata) == 0 {
		return nil
	}
	var parsed pluginExtraMetadata
	if err := json.Unmarshal(itemMeta.ExtraMetadata, &parsed); err != nil {
		return fmt.Errorf("parse extra_metadata: %w", err)
	}
	if len(parsed.ImCommands) == 0 {
		return nil
	}
	manifest := map[string]any{
		"id":       firstNonEmpty(itemMeta.Slug, pluginID),
		"version":  itemMeta.LatestVersion,
		"name":     itemMeta.Name,
		"commands": parsed.ImCommands,
	}
	if len(parsed.Tenants) > 0 {
		manifest["tenants"] = parsed.Tenants
	}
	body, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal plugin manifest: %w", err)
	}
	targetDir := filepath.Join(h.imBridgePluginDir, firstNonEmpty(itemMeta.Slug, pluginID))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir plugin dir: %w", err)
	}
	manifestPath := filepath.Join(targetDir, "plugin.yaml")
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		return fmt.Errorf("write plugin manifest: %w", err)
	}
	log.Infof("shipped marketplace im_commands to %s", manifestPath)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

func (h *MarketplaceHandler) installRoleArtifact(
	itemMeta *marketplaceItemMetadata,
	destDir string,
	installState *model.MarketplaceConsumptionRecord,
) error {
	if strings.TrimSpace(h.rolesDir) == "" {
		return fmt.Errorf("role install root is not configured")
	}
	stagingDir, err := h.extractZipPackage(filepath.Join(destDir, "artifact"), h.rolesDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingDir)

	roleManifestPath := filepath.Join(stagingDir, "role.yaml")
	manifest, err := rolepkg.ParseFile(roleManifestPath)
	if err != nil {
		return &marketplaceArtifactError{message: fmt.Sprintf("invalid role artifact: %v", err)}
	}
	if strings.TrimSpace(manifest.Metadata.ID) != "" && manifest.Metadata.ID != itemMeta.Slug {
		return &marketplaceArtifactError{message: "invalid role artifact: metadata.id must match marketplace item slug"}
	}

	targetDir := filepath.Join(h.rolesDir, itemMeta.Slug)
	if err := replaceDirectory(targetDir, stagingDir); err != nil {
		return err
	}
	if _, err := rolepkg.NewFileStore(h.rolesDir).Get(itemMeta.Slug); err != nil {
		_ = os.RemoveAll(targetDir)
		return &marketplaceArtifactError{message: fmt.Sprintf("invalid role artifact: failed discovery after install: %v", err)}
	}

	installState.Status = model.MarketplaceConsumptionStatusInstalled
	installState.Installed = true
	installState.Used = true
	installState.RecordID = itemMeta.Slug
	installState.LocalPath = targetDir
	installState.Provenance.RecordID = itemMeta.Slug
	installState.Provenance.LocalPath = targetDir
	return nil
}

func (h *MarketplaceHandler) installSkillArtifact(
	itemMeta *marketplaceItemMetadata,
	destDir string,
	installState *model.MarketplaceConsumptionRecord,
) error {
	skillsRoot := h.skillsRootDir()
	if strings.TrimSpace(skillsRoot) == "" {
		return fmt.Errorf("skill install root is not configured")
	}
	stagingDir, err := h.extractZipPackage(filepath.Join(destDir, "artifact"), skillsRoot)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stagingDir)

	if _, err := os.Stat(filepath.Join(stagingDir, "SKILL.md")); err != nil {
		return &marketplaceArtifactError{message: "invalid skill artifact: SKILL.md is required at the package root"}
	}

	targetDir := filepath.Join(skillsRoot, itemMeta.Slug)
	if err := replaceDirectory(targetDir, stagingDir); err != nil {
		return err
	}
	entries, err := rolepkg.DiscoverSkillCatalog(skillsRoot)
	if err != nil {
		_ = os.RemoveAll(targetDir)
		return &marketplaceArtifactError{message: fmt.Sprintf("invalid skill artifact: failed discovery after install: %v", err)}
	}
	expectedPath := "skills/" + itemMeta.Slug
	found := false
	for _, entry := range entries {
		if entry.Path == expectedPath {
			found = true
			break
		}
	}
	if !found {
		_ = os.RemoveAll(targetDir)
		return &marketplaceArtifactError{message: "invalid skill artifact: installed package is not discoverable from the role skill catalog"}
	}

	installState.Status = model.MarketplaceConsumptionStatusInstalled
	installState.Installed = true
	installState.Used = true
	installState.RecordID = itemMeta.Slug
	installState.LocalPath = targetDir
	installState.Provenance.RecordID = itemMeta.Slug
	installState.Provenance.LocalPath = targetDir
	return nil
}

func (h *MarketplaceHandler) installWorkflowTemplateArtifact(
	ctx context.Context,
	itemMeta *marketplaceItemMetadata,
	destDir string,
	installState *model.MarketplaceConsumptionRecord,
) error {
	if h.workflowTemplateRepo == nil {
		return fmt.Errorf("workflow template install not available: repo not configured")
	}

	stagingDir, err := h.extractZipPackage(filepath.Join(destDir, "artifact"), destDir)
	if err != nil {
		return err
	}

	// Find workflow.json in staging dir
	workflowFile := filepath.Join(stagingDir, "workflow.json")
	if _, err := os.Stat(workflowFile); os.IsNotExist(err) {
		// Try in subdirectories
		entries, _ := os.ReadDir(stagingDir)
		for _, e := range entries {
			if e.IsDir() {
				candidate := filepath.Join(stagingDir, e.Name(), "workflow.json")
				if _, err := os.Stat(candidate); err == nil {
					workflowFile = candidate
					break
				}
			}
		}
	}

	data, err := os.ReadFile(workflowFile)
	if err != nil {
		return fmt.Errorf("read workflow.json: %w", err)
	}

	var pkg service.WorkflowTemplatePackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("parse workflow.json: %w", err)
	}
	if pkg.Kind != "WorkflowTemplate" {
		return fmt.Errorf("invalid workflow package kind: %s", pkg.Kind)
	}

	// Import as a marketplace template (project-agnostic, using uuid.Nil)
	def, err := service.ImportTemplate(ctx, h.workflowTemplateRepo, uuid.Nil, &pkg)
	if err != nil {
		return fmt.Errorf("import workflow template: %w", err)
	}

	installState.Status = model.MarketplaceConsumptionStatusInstalled
	installState.Installed = true
	installState.Used = true
	installState.RecordID = def.ID.String()
	installState.LocalPath = destDir
	installState.Provenance.RecordID = def.ID.String()
	installState.Provenance.LocalPath = destDir
	return nil
}

func (h *MarketplaceHandler) extractZipPackage(artifactPath string, tempParent string) (string, error) {
	reader, err := zip.OpenReader(artifactPath)
	if err != nil {
		return "", &marketplaceArtifactError{message: fmt.Sprintf("invalid artifact: expected zip archive: %v", err)}
	}
	defer reader.Close()

	if err := os.MkdirAll(tempParent, 0o755); err != nil {
		return "", err
	}
	stagingDir, err := os.MkdirTemp(tempParent, ".marketplace-package-*")
	if err != nil {
		return "", err
	}

	for _, file := range reader.File {
		name := filepath.Clean(file.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			_ = os.RemoveAll(stagingDir)
			return "", &marketplaceArtifactError{message: "invalid artifact: archive contains unsafe paths"}
		}
		if strings.Contains(name, string(os.PathSeparator)+"..") {
			_ = os.RemoveAll(stagingDir)
			return "", &marketplaceArtifactError{message: "invalid artifact: archive contains unsafe paths"}
		}

		targetPath := filepath.Join(stagingDir, name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				_ = os.RemoveAll(stagingDir)
				return "", err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			_ = os.RemoveAll(stagingDir)
			return "", err
		}
		src, err := file.Open()
		if err != nil {
			_ = os.RemoveAll(stagingDir)
			return "", err
		}
		dst, err := os.Create(targetPath)
		if err != nil {
			src.Close()
			_ = os.RemoveAll(stagingDir)
			return "", err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			src.Close()
			_ = os.RemoveAll(stagingDir)
			return "", err
		}
		dst.Close()
		src.Close()
	}

	packageRoot, err := normalizeExtractedPackageRoot(stagingDir)
	if err != nil {
		_ = os.RemoveAll(stagingDir)
		return "", err
	}

	return packageRoot, nil
}

func replaceDirectory(targetDir string, sourceDir string) error {
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return err
	}
	return os.Rename(sourceDir, targetDir)
}

func normalizeExtractedPackageRoot(stagingDir string) (string, error) {
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return "", err
	}

	nonHidden := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		nonHidden = append(nonHidden, entry)
	}
	if len(nonHidden) != 1 || !nonHidden[0].IsDir() {
		return stagingDir, nil
	}

	rootDir := filepath.Join(stagingDir, nonHidden[0].Name())
	relocatedDir := stagingDir + "-root"
	if err := os.Rename(rootDir, relocatedDir); err != nil {
		return "", err
	}
	if err := os.RemoveAll(stagingDir); err != nil {
		_ = os.RemoveAll(relocatedDir)
		return "", err
	}
	return relocatedDir, nil
}

func (h *MarketplaceHandler) writeConsumptionState(record model.MarketplaceConsumptionRecord) error {
	statePath := filepath.Join(h.marketplaceRootDir(), record.ItemID, record.Version, marketplaceConsumptionStateFile)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0o644)
}

func (h *MarketplaceHandler) loadConsumptionStates() ([]model.MarketplaceConsumptionRecord, error) {
	root := h.marketplaceRootDir()
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	records := []model.MarketplaceConsumptionRecord{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Base(path) != marketplaceConsumptionStateFile {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var record model.MarketplaceConsumptionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return err
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func marketplaceItemType(itemType string) model.MarketplaceItemType {
	switch strings.TrimSpace(itemType) {
	case string(model.MarketplaceItemTypeSkill):
		return model.MarketplaceItemTypeSkill
	case string(model.MarketplaceItemTypeRole):
		return model.MarketplaceItemTypeRole
	case string(model.MarketplaceItemTypeWorkflowTemplate):
		return model.MarketplaceItemTypeWorkflowTemplate
	case string(model.MarketplaceItemTypeProjectTemplate):
		return model.MarketplaceItemTypeProjectTemplate
	default:
		return model.MarketplaceItemTypePlugin
	}
}

func marketplaceConsumerSurface(itemType string) model.MarketplaceConsumerSurface {
	switch marketplaceItemType(itemType) {
	case model.MarketplaceItemTypeRole:
		return model.MarketplaceConsumerSurfaceRoleWorkspace
	case model.MarketplaceItemTypeSkill:
		return model.MarketplaceConsumerSurfaceRoleSkillCatalog
	case model.MarketplaceItemTypeWorkflowTemplate:
		return model.MarketplaceConsumerSurfaceWorkflowTemplateLibrary
	case model.MarketplaceItemTypeProjectTemplate:
		return model.MarketplaceConsumerSurfaceProjectTemplateLibrary
	default:
		return model.MarketplaceConsumerSurfacePluginManagementPanel
	}
}

func marketplaceConsumptionKey(itemID string, version string) string {
	return itemID + "@" + version
}

// MarketplaceUninstallRequest is the request body for POST /marketplace/uninstall.
type MarketplaceUninstallRequest struct {
	ItemID   string `json:"item_id" validate:"required"`
	ItemType string `json:"item_type" validate:"required"`
}

// Uninstall removes a marketplace-installed item and cleans up its consumption state.
func (h *MarketplaceHandler) Uninstall(c echo.Context) error {
	req := new(MarketplaceUninstallRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request"})
	}
	if req.ItemID == "" || req.ItemType == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "item_id and item_type are required"})
	}

	ctx := c.Request().Context()
	itemType := marketplaceItemType(req.ItemType)

	// Find existing consumption records for this item.
	records, err := h.loadConsumptionStates()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to load marketplace state"})
	}

	var matched *model.MarketplaceConsumptionRecord
	for i := range records {
		if records[i].ItemID == req.ItemID {
			matched = &records[i]
			break
		}
	}
	if matched == nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "no consumption record found for this item"})
	}

	switch itemType {
	case model.MarketplaceItemTypePlugin:
		// Uninstall via plugin service if a record ID exists.
		if matched.RecordID != "" {
			if err := h.pluginSvc.Uninstall(ctx, matched.RecordID); err != nil {
				return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: fmt.Sprintf("failed to uninstall plugin: %v", err)})
			}
		}
	case model.MarketplaceItemTypeRole:
		if matched.LocalPath != "" {
			_ = os.RemoveAll(matched.LocalPath)
		}
	case model.MarketplaceItemTypeSkill:
		if matched.LocalPath != "" {
			_ = os.RemoveAll(matched.LocalPath)
		}
	}

	// Remove the marketplace consumption directory for all versions.
	marketplaceItemDir := filepath.Join(h.marketplaceRootDir(), req.ItemID)
	_ = os.RemoveAll(marketplaceItemDir)

	return c.JSON(http.StatusOK, map[string]any{
		"ok":      true,
		"itemId":  req.ItemID,
		"message": "item uninstalled",
	})
}

// Sideload accepts a local zip artifact upload and installs it as a marketplace-like item.
func (h *MarketplaceHandler) Sideload(c echo.Context) error {
	itemType := strings.TrimSpace(c.FormValue("type"))
	if itemType == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "type is required (plugin, role, skill, or workflow_template)"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "file is required"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to open uploaded file"})
	}
	defer src.Close()

	// Save to temp file.
	tmpFile, err := os.CreateTemp("", "marketplace-sideload-*.zip")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create temp file"})
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, src); err != nil {
		tmpFile.Close()
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to save uploaded file"})
	}
	tmpFile.Close()

	ctx := c.Request().Context()
	mItemType := marketplaceItemType(itemType)

	// Derive a slug from the filename (strip extension).
	slug := strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
	slug = strings.TrimSuffix(slug, ".tar") // handle .tar.gz etc.
	if slug == "" {
		slug = "sideload-" + fmt.Sprintf("%d", time.Now().UnixMilli())
	}

	installState := model.MarketplaceConsumptionRecord{
		ItemID:          "sideload-" + slug,
		ItemType:        mItemType,
		Version:         "local",
		Status:          model.MarketplaceConsumptionStatusBlocked,
		ConsumerSurface: marketplaceConsumerSurface(itemType),
		Installed:       false,
		Used:            false,
		UpdatedAt:       time.Now().UTC(),
		Provenance: &model.MarketplaceConsumptionProvenance{
			SourceType:        "sideload",
			MarketplaceItemID: "sideload-" + slug,
			SelectedVersion:   "local",
		},
	}

	switch mItemType {
	case model.MarketplaceItemTypeRole:
		if strings.TrimSpace(h.rolesDir) == "" {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "role install root is not configured"})
		}
		stagingDir, err := h.extractZipPackage(tmpPath, h.rolesDir)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("invalid artifact: %v", err)})
		}
		defer os.RemoveAll(stagingDir)

		roleManifestPath := filepath.Join(stagingDir, "role.yaml")
		manifest, err := rolepkg.ParseFile(roleManifestPath)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("invalid role artifact: %v", err)})
		}
		if strings.TrimSpace(manifest.Metadata.ID) != "" {
			slug = manifest.Metadata.ID
			installState.ItemID = "sideload-" + slug
			installState.Provenance.MarketplaceItemID = installState.ItemID
		}

		targetDir := filepath.Join(h.rolesDir, slug)
		if err := replaceDirectory(targetDir, stagingDir); err != nil {
			return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to install role"})
		}
		if _, err := rolepkg.NewFileStore(h.rolesDir).Get(slug); err != nil {
			_ = os.RemoveAll(targetDir)
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("invalid role artifact: %v", err)})
		}
		installState.Status = model.MarketplaceConsumptionStatusInstalled
		installState.Installed = true
		installState.Used = true
		installState.RecordID = slug
		installState.LocalPath = targetDir
		installState.Provenance.RecordID = slug
		installState.Provenance.LocalPath = targetDir

	case model.MarketplaceItemTypeSkill:
		skillsRoot := h.skillsRootDir()
		if strings.TrimSpace(skillsRoot) == "" {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "skill install root is not configured"})
		}
		stagingDir, err := h.extractZipPackage(tmpPath, skillsRoot)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("invalid artifact: %v", err)})
		}
		defer os.RemoveAll(stagingDir)

		if _, err := os.Stat(filepath.Join(stagingDir, "SKILL.md")); err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid skill artifact: SKILL.md is required at the package root"})
		}

		targetDir := filepath.Join(skillsRoot, slug)
		if err := replaceDirectory(targetDir, stagingDir); err != nil {
			return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to install skill"})
		}
		entries, err := rolepkg.DiscoverSkillCatalog(skillsRoot)
		if err != nil {
			_ = os.RemoveAll(targetDir)
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("invalid skill artifact: %v", err)})
		}
		expectedPath := "skills/" + slug
		found := false
		for _, entry := range entries {
			if entry.Path == expectedPath {
				found = true
				break
			}
		}
		if !found {
			_ = os.RemoveAll(targetDir)
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid skill artifact: installed package is not discoverable"})
		}
		installState.Status = model.MarketplaceConsumptionStatusInstalled
		installState.Installed = true
		installState.Used = true
		installState.RecordID = slug
		installState.LocalPath = targetDir
		installState.Provenance.RecordID = slug
		installState.Provenance.LocalPath = targetDir

	case model.MarketplaceItemTypePlugin:
		// Delegate to existing plugin install seam.
		source := &model.PluginSource{
			Type: model.PluginSourceLocal,
			Path: tmpPath,
		}
		stagingDir, err := h.extractZipPackage(tmpPath, h.pluginsDir)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("invalid artifact: %v", err)})
		}
		targetDir := filepath.Join(h.pluginsDir, "sideload-"+slug)
		if err := replaceDirectory(targetDir, stagingDir); err != nil {
			return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to install plugin"})
		}
		source.Path = targetDir
		record, err := h.pluginSvc.Install(ctx, service.PluginInstallRequest{
			Path:   targetDir,
			Source: source,
		})
		if err != nil {
			_ = os.RemoveAll(targetDir)
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("failed to install plugin: %v", err)})
		}
		installState.Status = model.MarketplaceConsumptionStatusInstalled
		installState.Installed = true
		installState.Used = true
		installState.RecordID = record.Metadata.ID
		installState.LocalPath = targetDir
		installState.Provenance.RecordID = record.Metadata.ID
		installState.Provenance.LocalPath = targetDir

	case model.MarketplaceItemTypeWorkflowTemplate:
		if h.workflowTemplateRepo == nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "workflow template install not available"})
		}
		stagingDir, err := h.extractZipPackage(tmpPath, os.TempDir())
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("invalid artifact: %v", err)})
		}
		defer os.RemoveAll(stagingDir)

		workflowFile := filepath.Join(stagingDir, "workflow.json")
		data, err := os.ReadFile(workflowFile)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid workflow template: workflow.json is required"})
		}
		var pkg service.WorkflowTemplatePackage
		if err := json.Unmarshal(data, &pkg); err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("invalid workflow.json: %v", err)})
		}
		def, err := service.ImportTemplate(ctx, h.workflowTemplateRepo, uuid.Nil, &pkg)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: fmt.Sprintf("failed to import template: %v", err)})
		}
		installState.Status = model.MarketplaceConsumptionStatusInstalled
		installState.Installed = true
		installState.Used = true
		installState.RecordID = def.ID.String()
		installState.Provenance.RecordID = def.ID.String()

	default:
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "unsupported item type: " + itemType})
	}

	if err := h.writeConsumptionState(installState); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to persist marketplace state"})
	}

	return c.JSON(http.StatusOK, model.MarketplaceInstallResponse{
		OK:   true,
		Item: installState,
	})
}

// MarketplaceUpdateInfo represents a single item's update availability.
type MarketplaceUpdateInfo struct {
	ItemID           string `json:"itemId"`
	ItemType         string `json:"itemType"`
	InstalledVersion string `json:"installedVersion"`
	LatestVersion    string `json:"latestVersion"`
	HasUpdate        bool   `json:"hasUpdate"`
}

// Updates checks all installed marketplace items against the marketplace service for newer versions.
func (h *MarketplaceHandler) Updates(c echo.Context) error {
	if strings.TrimSpace(h.marketURL) == "" {
		return c.JSON(http.StatusOK, map[string]any{"items": []MarketplaceUpdateInfo{}})
	}

	records, err := h.loadConsumptionStates()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to load marketplace state"})
	}

	authHeader := c.Request().Header.Get("Authorization")
	ctx := c.Request().Context()
	var updates []MarketplaceUpdateInfo

	for _, record := range records {
		if record.Status != model.MarketplaceConsumptionStatusInstalled {
			continue
		}
		if record.Provenance == nil || record.Provenance.SourceType == "sideload" || record.Provenance.SourceType == string(model.PluginSourceBuiltin) {
			continue
		}
		if strings.TrimSpace(record.Version) == "" || record.Version == "local" {
			continue
		}

		meta, err := h.fetchItemMetadata(ctx, authHeader, record.ItemID)
		if err != nil {
			continue
		}

		latestVersion := meta.LatestVersion
		if latestVersion == "" {
			continue
		}

		updates = append(updates, MarketplaceUpdateInfo{
			ItemID:           record.ItemID,
			ItemType:         string(record.ItemType),
			InstalledVersion: record.Version,
			LatestVersion:    latestVersion,
			HasUpdate:        latestVersion != record.Version,
		})
	}

	if updates == nil {
		updates = []MarketplaceUpdateInfo{}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": updates})
}

// --- project template marketplace install seam ---

// marketplaceInstallerUserIDKey is the ctx key used to hand the installer's
// user id from the Install entrypoint down to sub-install helpers that need
// to know who owns the materialized row.
type marketplaceInstallerUserIDKey struct{}

func withMarketplaceInstallerUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, marketplaceInstallerUserIDKey{}, id)
}

func marketplaceInstallerUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(marketplaceInstallerUserIDKey{}).(uuid.UUID)
	if !ok || v == uuid.Nil {
		return uuid.Nil, false
	}
	return v, true
}

// projectTemplateArtifact is the on-disk payload shape we expect inside a
// marketplace `project_template` package: either a bare JSON file named
// project-template.json at the root of the zip, or the same file under a
// single subdirectory (mirrors the workflow template layout).
type projectTemplateArtifact struct {
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	SnapshotVersion int             `json:"snapshotVersion"`
	Snapshot        json.RawMessage `json:"snapshot"`
}

func (h *MarketplaceHandler) installProjectTemplateArtifact(
	ctx context.Context,
	itemMeta *marketplaceItemMetadata,
	destDir string,
	installState *model.MarketplaceConsumptionRecord,
) error {
	if h.projectTemplateInst == nil {
		return fmt.Errorf("project template install not available: installer not configured")
	}
	userID, ok := marketplaceInstallerUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("project template install: missing installer user context")
	}

	stagingDir, err := h.extractZipPackage(filepath.Join(destDir, "artifact"), destDir)
	if err != nil {
		return err
	}
	templateFile := filepath.Join(stagingDir, "project-template.json")
	if _, statErr := os.Stat(templateFile); os.IsNotExist(statErr) {
		entries, _ := os.ReadDir(stagingDir)
		for _, e := range entries {
			if e.IsDir() {
				candidate := filepath.Join(stagingDir, e.Name(), "project-template.json")
				if _, err := os.Stat(candidate); err == nil {
					templateFile = candidate
					break
				}
			}
		}
	}
	data, err := os.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("read project-template.json: %w", err)
	}
	var pkg projectTemplateArtifact
	if err := json.Unmarshal(data, &pkg); err != nil {
		return &marketplaceArtifactError{message: fmt.Sprintf("parse project-template.json: %v", err)}
	}
	name := strings.TrimSpace(pkg.Name)
	if name == "" {
		name = strings.TrimSpace(itemMeta.Name)
	}
	if name == "" {
		return &marketplaceArtifactError{message: "project template has no name"}
	}
	snapshot := strings.TrimSpace(string(pkg.Snapshot))
	if snapshot == "" {
		return &marketplaceArtifactError{message: "project template snapshot is empty"}
	}
	tpl, err := h.projectTemplateInst.MaterializeMarketplaceInstall(
		ctx, userID, name, pkg.Description, snapshot, pkg.SnapshotVersion,
	)
	if err != nil {
		return fmt.Errorf("materialize project template: %w", err)
	}
	installState.Status = model.MarketplaceConsumptionStatusInstalled
	installState.Installed = true
	installState.Used = true
	installState.RecordID = tpl.ID.String()
	installState.LocalPath = destDir
	installState.Provenance.RecordID = tpl.ID.String()
	installState.Provenance.LocalPath = destDir
	return nil
}
