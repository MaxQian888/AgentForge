package qchandler

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
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
