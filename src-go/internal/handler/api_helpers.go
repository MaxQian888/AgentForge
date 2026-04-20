package handler

import (
	"strings"
	"time"

	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func claimsUserID(c echo.Context) (*uuid.UUID, error) {
	claims, err := appMiddleware.GetClaims(c)
	if err != nil {
		return nil, err
	}
	parsed, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseOptionalUUIDString(value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseOptionalTimeString(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
