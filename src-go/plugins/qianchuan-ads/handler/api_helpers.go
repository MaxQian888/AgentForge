package qchandler

import (
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// claimsUserID mirrors the unexported helper of the same name in
// internal/handler. Kept verbatim so the plugin doesn't take a
// dependency on a private core helper.
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
