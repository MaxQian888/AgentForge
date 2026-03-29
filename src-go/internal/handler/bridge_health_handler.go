package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/service"
)

type BridgeHealthStatusReader interface {
	Snapshot() service.BridgeHealthSnapshot
}

type BridgeHealthHandler struct {
	health BridgeHealthStatusReader
}

func NewBridgeHealthHandler(health BridgeHealthStatusReader) *BridgeHealthHandler {
	return &BridgeHealthHandler{health: health}
}

func (h *BridgeHealthHandler) Get(c echo.Context) error {
	if h.health == nil {
		return c.JSON(http.StatusServiceUnavailable, service.BridgeHealthSnapshot{Status: service.BridgeStatusDegraded})
	}
	return c.JSON(http.StatusOK, h.health.Snapshot())
}
