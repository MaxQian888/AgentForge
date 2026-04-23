package middleware

import (
	"net/http"
	"strings"

	"github.com/agentforge/server/internal/authutil"
	appI18n "github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/labstack/echo/v4"
)

// ReviewTriggerAuthMiddleware accepts either a service token for GitHub workflows
// or a normal JWT Bearer token for authenticated in-product callers.
func ReviewTriggerAuthMiddleware(secret string, blacklist TokenBlacklist, serviceToken string) echo.MiddlewareFunc {
	jwtMiddleware := JWTMiddleware(secret, blacklist)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgMissingAuthHeader)})
			}

			if err := authutil.ValidateBearerSharedSecret(authHeader, serviceToken); err == nil {
				return next(c)
			}

			return jwtMiddleware(next)(c)
		}
	}
}
