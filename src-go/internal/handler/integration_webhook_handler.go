package handler

import (
	"context"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// IntegrationPluginInvoker is satisfied by the existing plugin runtime
// (PluginService.Invoke). The handler defines its own narrow interface so
// tests can supply a stub without dragging in the full plugin runtime.
type IntegrationPluginInvoker interface {
	InvokeIntegrationPlugin(ctx context.Context, pluginID, operation string, payload map[string]any) (map[string]any, error)
}

// WebhookEventPublisher publishes a typed event to the EventBus. The handler
// uses a raw type+payload shape because plugins return a generic event
// envelope; the wiring layer adapts this to eventbus.Publisher.Publish.
type WebhookEventPublisher interface {
	PublishRaw(ctx context.Context, eventType string, payload map[string]any) error
}

// IntegrationWebhookHandler serves POST /api/v1/integrations/:id/webhook.
// It reads the raw request body and headers, hands them to the integration
// plugin's "handle_webhook" operation, then publishes the returned event
// (if any) to the EventBus. The plugin is responsible for any signature
// validation; this endpoint must remain unauthenticated so external systems
// (GitHub, GitLab, etc.) can post to it.
type IntegrationWebhookHandler struct {
	invoker   IntegrationPluginInvoker
	publisher WebhookEventPublisher
}

func NewIntegrationWebhookHandler(invoker IntegrationPluginInvoker, publisher WebhookEventPublisher) *IntegrationWebhookHandler {
	return &IntegrationWebhookHandler{invoker: invoker, publisher: publisher}
}

func (h *IntegrationWebhookHandler) Handle(c echo.Context) error {
	pluginID := c.Param("id")
	ctx := c.Request().Context()

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot read body"})
	}

	headers := make(map[string]string, len(c.Request().Header))
	for k, v := range c.Request().Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	payload := map[string]any{
		"body":    string(body),
		"headers": headers,
	}

	result, err := h.invoker.InvokeIntegrationPlugin(ctx, pluginID, "handle_webhook", payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if result != nil {
		if eventType, ok := result["event_type"].(string); ok && eventType != "" {
			eventPayload, _ := result["payload"].(map[string]any)
			if eventPayload == nil {
				eventPayload = map[string]any{}
			}
			if pubErr := h.publisher.PublishRaw(ctx, eventType, eventPayload); pubErr != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": pubErr.Error()})
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
