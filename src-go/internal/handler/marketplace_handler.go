package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

// MarketplaceHandler handles marketplace install integration endpoints on the src-go backend.
type MarketplaceHandler struct {
	pluginSvc  *service.PluginService
	marketURL  string
	httpClient *http.Client
	pluginsDir string
}

// NewMarketplaceHandler creates a new MarketplaceHandler.
func NewMarketplaceHandler(pluginSvc *service.PluginService, marketURL string, pluginsDir string) *MarketplaceHandler {
	return &MarketplaceHandler{
		pluginSvc:  pluginSvc,
		marketURL:  marketURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		pluginsDir: pluginsDir,
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
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to reach marketplace service"})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "marketplace service returned error"})
	}

	expectedDigest := resp.Header.Get("X-Content-Digest")

	// 2. Save artifact while computing SHA-256.
	destDir := filepath.Join(h.pluginsDir, "marketplace", req.ItemID, req.Version)
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
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to download artifact"})
	}
	f.Close()

	// 3. Verify digest.
	actualDigest := hex.EncodeToString(hasher.Sum(nil))
	if expectedDigest != "" && actualDigest != expectedDigest {
		os.Remove(tmpPath)
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "artifact digest mismatch"})
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to save artifact"})
	}

	// 4. Fetch item metadata from marketplace service.
	metaURL := fmt.Sprintf("%s/api/v1/items/%s", h.marketURL, req.ItemID)
	metaReq, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create metadata request"})
	}
	metaReq.Header.Set("Authorization", authHeader)

	metaResp, err := h.httpClient.Do(metaReq)
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to fetch item metadata"})
	}
	defer metaResp.Body.Close()

	var itemMeta struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(metaResp.Body).Decode(&itemMeta); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to parse item metadata"})
	}

	// 5. Register in plugin service based on item type.
	switch itemMeta.Type {
	case "plugin":
		source := &model.PluginSource{
			Type:    model.PluginSourceMarketplace,
			Catalog: req.ItemID,
			Ref:     req.Version,
			Path:    destDir,
		}
		_, err := h.pluginSvc.Install(ctx, service.PluginInstallRequest{
			Path:   destDir,
			Source: source,
		})
		if err != nil {
			// Artifact is already saved; return a warning rather than failing.
			return c.JSON(http.StatusOK, map[string]interface{}{
				"ok":      true,
				"item_id": req.ItemID,
				"version": req.Version,
				"type":    itemMeta.Type,
				"warning": err.Error(),
			})
		}
	// For skills and roles, the artifact is stored on disk; full extraction
	// would happen in a future implementation.
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"ok":      true,
		"item_id": req.ItemID,
		"version": req.Version,
		"type":    itemMeta.Type,
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
