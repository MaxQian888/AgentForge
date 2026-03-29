package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	bridge "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
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
	Generate(ctx context.Context, req bridge.GenerateRequest) (*bridge.GenerateResponse, error)
	ClassifyIntent(ctx context.Context, req bridge.ClassifyIntentRequest) (*bridge.ClassifyIntentResponse, error)
}

type bridgeAIContextAdapter struct {
	client *bridge.Client
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
