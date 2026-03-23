package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type HealthHandler struct {
	version   string
	commit    string
	buildDate string
	env       string
}

func NewHealthHandler(version, commit, buildDate, env string) *HealthHandler {
	return &HealthHandler{version: version, commit: commit, buildDate: buildDate, env: env}
}

func (h *HealthHandler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *HealthHandler) HealthV1(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":    "ok",
		"version":   h.version,
		"commit":    h.commit,
		"buildDate": h.buildDate,
		"env":       h.env,
		"time":      time.Now().UTC().Format(time.RFC3339),
	})
}
