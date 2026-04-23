package middleware

import (
	"errors"
	"net/http"

	"github.com/agentforge/server/internal/authutil"
	"github.com/agentforge/server/internal/model"
	"github.com/labstack/echo/v4"
)

// SharedSecretAuthMiddleware protects internal routes with a Bearer shared
// secret. Misconfigured secrets fail closed instead of silently leaving the
// route open.
func SharedSecretAuthMiddleware(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := authutil.ValidateBearerSharedSecret(c.Request().Header.Get("Authorization"), secret)
			switch {
			case err == nil:
				return next(c)
			case errors.Is(err, authutil.ErrSharedSecretNotConfigured):
				return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "internal auth is not configured"})
			default:
				return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "unauthorized"})
			}
		}
	}
}
