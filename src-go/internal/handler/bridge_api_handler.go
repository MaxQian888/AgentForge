package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	bridge "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	pluginparser "github.com/react-go-quick-starter/server/internal/plugin"
)

type BridgeRuntimeCatalogHandler struct {
	client BridgeRuntimeCatalogClient
	ttl    time.Duration
	now    func() time.Time

	mu       sync.Mutex
	cachedAt time.Time
	cached   *bridge.RuntimeCatalogResponse
}

type BridgeRuntimeCatalogClient interface {
	GetRuntimeCatalog(ctx context.Context) (*bridge.RuntimeCatalogResponse, error)
}

type bridgeRuntimeCatalogContextAdapter struct {
	client *bridge.Client
}

func (a bridgeRuntimeCatalogContextAdapter) GetRuntimeCatalog(ctx context.Context) (*bridge.RuntimeCatalogResponse, error) {
	return a.client.GetRuntimeCatalog(ctx)
}

func NewBridgeRuntimeCatalogHandler(client *bridge.Client) *BridgeRuntimeCatalogHandler {
	return newBridgeRuntimeCatalogHandlerWithConfig(bridgeRuntimeCatalogContextAdapter{client: client}, 60*time.Second, time.Now)
}

func newBridgeRuntimeCatalogHandlerWithConfig(client BridgeRuntimeCatalogClient, ttl time.Duration, now func() time.Time) *BridgeRuntimeCatalogHandler {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	if now == nil {
		now = time.Now
	}
	return &BridgeRuntimeCatalogHandler{client: client, ttl: ttl, now: now}
}

func (h *BridgeRuntimeCatalogHandler) Get(c echo.Context) error {
	if h.client == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "bridge runtime catalog unavailable"})
	}

	h.mu.Lock()
	if h.cached != nil && h.now().Sub(h.cachedAt) < h.ttl {
		cached := h.cached
		h.mu.Unlock()
		return c.JSON(http.StatusOK, cached)
	}
	h.mu.Unlock()

	resp, err := h.client.GetRuntimeCatalog(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: err.Error()})
	}

	h.mu.Lock()
	h.cached = resp
	h.cachedAt = h.now()
	h.mu.Unlock()
	return c.JSON(http.StatusOK, resp)
}

type BridgeAIHandler struct {
	client BridgeAIClient
}

type BridgeAIClient interface {
	DecomposeTask(ctx context.Context, req bridge.DecomposeRequest) (*bridge.DecomposeResponse, error)
	Generate(ctx context.Context, req bridge.GenerateRequest) (*bridge.GenerateResponse, error)
	ClassifyIntent(ctx context.Context, req bridge.ClassifyIntentRequest) (*bridge.ClassifyIntentResponse, error)
}

type bridgeAIContextAdapter struct {
	client *bridge.Client
}

func (a bridgeAIContextAdapter) DecomposeTask(ctx context.Context, req bridge.DecomposeRequest) (*bridge.DecomposeResponse, error) {
	return a.client.DecomposeTask(ctx, req)
}

func (a bridgeAIContextAdapter) Generate(ctx context.Context, req bridge.GenerateRequest) (*bridge.GenerateResponse, error) {
	return a.client.Generate(ctx, req)
}

func (a bridgeAIContextAdapter) ClassifyIntent(ctx context.Context, req bridge.ClassifyIntentRequest) (*bridge.ClassifyIntentResponse, error) {
	return a.client.ClassifyIntent(ctx, req)
}

func NewBridgeAIHandler(client BridgeAIClient) *BridgeAIHandler {
	if concrete, ok := client.(*bridge.Client); ok {
		return &BridgeAIHandler{client: bridgeAIContextAdapter{client: concrete}}
	}
	return &BridgeAIHandler{client: client}
}

type bridgeGenerateRequest struct {
	Prompt   string `json:"prompt" validate:"required"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type bridgeClassifyIntentRequest struct {
	Text       string   `json:"text" validate:"required"`
	Candidates []string `json:"candidates"`
	UserID     string   `json:"user_id"`
	ProjectID  string   `json:"project_id"`
}

type bridgeDecomposeRequest struct {
	TaskID      string `json:"task_id" validate:"required"`
	Title       string `json:"title" validate:"required"`
	Description string `json:"description" validate:"required"`
	Priority    string `json:"priority" validate:"required"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Context     any    `json:"context,omitempty"`
}

func (h *BridgeAIHandler) Generate(c echo.Context) error {
	if h.client == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "bridge ai unavailable"})
	}

	req := new(bridgeGenerateRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.client.Generate(c.Request().Context(), bridge.GenerateRequest{
		Prompt:   req.Prompt,
		Provider: req.Provider,
		Model:    req.Model,
	})
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *BridgeAIHandler) Decompose(c echo.Context) error {
	if h.client == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "bridge ai unavailable"})
	}

	req := new(bridgeDecomposeRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.client.DecomposeTask(c.Request().Context(), bridge.DecomposeRequest{
		TaskID:      req.TaskID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Provider:    req.Provider,
		Model:       req.Model,
		Context:     req.Context,
	})
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *BridgeAIHandler) ClassifyIntent(c echo.Context) error {
	if h.client == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "bridge ai unavailable"})
	}

	req := new(bridgeClassifyIntentRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.client.ClassifyIntent(c.Request().Context(), bridge.ClassifyIntentRequest{
		Text:      req.Text,
		UserID:    req.UserID,
		ProjectID: req.ProjectID,
	})
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

type BridgePoolClient interface {
	GetPool(ctx context.Context) (*bridge.PoolSummaryResponse, error)
}

type BridgePoolHandler struct {
	client BridgePoolClient
}

func NewBridgePoolHandler(client BridgePoolClient) *BridgePoolHandler {
	return &BridgePoolHandler{client: client}
}

func (h *BridgePoolHandler) Get(c echo.Context) error {
	if h.client == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "bridge pool unavailable"})
	}

	resp, err := h.client.GetPool(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

type BridgeToolsClient interface {
	ListTools(ctx context.Context) (*bridge.ToolsListResponse, error)
	InstallTool(ctx context.Context, manifest model.PluginManifest) (*model.PluginRecord, error)
	UninstallTool(ctx context.Context, pluginID string) (*model.PluginRecord, error)
	RestartTool(ctx context.Context, pluginID string) (*model.PluginRecord, error)
}

type BridgeToolsHandler struct {
	client               BridgeToolsClient
	httpClient           *http.Client
	allowedManifestHosts map[string]struct{}
}

func NewBridgeToolsHandler(client BridgeToolsClient, allowedManifestHosts ...string) *BridgeToolsHandler {
	hostSet := make(map[string]struct{})
	for _, entry := range allowedManifestHosts {
		if host := normalizeManifestHost(entry); host != "" {
			hostSet[host] = struct{}{}
		}
	}
	return &BridgeToolsHandler{
		client:               client,
		httpClient:           &http.Client{Timeout: 15 * time.Second},
		allowedManifestHosts: hostSet,
	}
}

type bridgeToolMutationRequest struct {
	PluginID string `json:"plugin_id" validate:"required"`
}

type bridgeToolInstallRequest struct {
	ManifestURL string `json:"manifest_url" validate:"required,url"`
}

func (h *BridgeToolsHandler) List(c echo.Context) error {
	if h.client == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "bridge tools unavailable"})
	}

	resp, err := h.client.ListTools(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *BridgeToolsHandler) Install(c echo.Context) error {
	if h.client == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "bridge tools unavailable"})
	}

	req := new(bridgeToolInstallRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	manifest, statusCode, err := h.fetchManifest(c.Request().Context(), strings.TrimSpace(req.ManifestURL))
	if err != nil {
		return c.JSON(statusCode, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.client.InstallTool(c.Request().Context(), *manifest)
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *BridgeToolsHandler) Uninstall(c echo.Context) error {
	if h.client == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "bridge tools unavailable"})
	}

	req := new(bridgeToolMutationRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.client.UninstallTool(c.Request().Context(), strings.TrimSpace(req.PluginID))
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *BridgeToolsHandler) Restart(c echo.Context) error {
	if h.client == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "bridge tools unavailable"})
	}

	pluginID := strings.TrimSpace(c.Param("id"))
	if pluginID == "" {
		req := new(bridgeToolMutationRequest)
		if err := c.Bind(req); err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
		}
		if err := c.Validate(req); err != nil {
			return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
		}
		pluginID = strings.TrimSpace(req.PluginID)
	}

	resp, err := h.client.RestartTool(c.Request().Context(), pluginID)
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *BridgeToolsHandler) fetchManifest(ctx context.Context, rawURL string) (*model.PluginManifest, int, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL == nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid manifest_url")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, http.StatusBadRequest, fmt.Errorf("manifest_url must use http or https")
	}
	if !h.isAllowedManifestHost(parsedURL.Hostname()) {
		return nil, http.StatusForbidden, fmt.Errorf("Manifest URL not in allowlist")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return nil, http.StatusBadGateway, fmt.Errorf("Failed to fetch manifest: %s", message)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	manifest, err := pluginparser.Parse(data)
	if err != nil {
		return nil, http.StatusUnprocessableEntity, err
	}
	return manifest, http.StatusOK, nil
}

func (h *BridgeToolsHandler) isAllowedManifestHost(host string) bool {
	host = normalizeManifestHost(host)
	if host == "" || len(h.allowedManifestHosts) == 0 {
		return false
	}
	_, ok := h.allowedManifestHosts[host]
	return ok
}

func normalizeManifestHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		if parsedURL, err := url.Parse(raw); err == nil {
			return strings.ToLower(strings.TrimSpace(parsedURL.Hostname()))
		}
	}
	if host, _, found := strings.Cut(raw, ":"); found && host != "" {
		raw = host
	}
	return strings.ToLower(strings.TrimSpace(raw))
}
