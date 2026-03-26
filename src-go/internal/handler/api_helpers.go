package handler

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
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
