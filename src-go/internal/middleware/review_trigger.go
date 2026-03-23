package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
)

// ReviewTriggerAuthMiddleware accepts either a service token for GitHub workflows
// or a normal JWT Bearer token for authenticated in-product callers.
func ReviewTriggerAuthMiddleware(secret string, blacklist TokenBlacklist, serviceToken string) echo.MiddlewareFunc {
	jwtMiddleware := JWTMiddleware(secret, blacklist)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "missing or invalid authorization header"})
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if serviceToken != "" && tokenStr == serviceToken {
				return next(c)
			}

			return jwtMiddleware(next)(c)
		}
	}
}
