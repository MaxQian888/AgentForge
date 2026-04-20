// Package middleware provides Echo middleware for authentication and request processing.
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	appI18n "github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// TokenBlacklist defines the interface for checking revoked tokens.
type TokenBlacklist interface {
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

// JWTContextKey is used to store parsed claims in echo.Context.
const JWTContextKey = "jwt_claims"

// JWTMiddleware validates the Bearer token and checks the blacklist.
func JWTMiddleware(secret string, blacklist TokenBlacklist) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgMissingAuthHeader)})
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims := &service.Claims{}

			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgInvalidOrExpiredToken)})
			}

			// Check blacklist
			blacklisted, err := blacklist.IsBlacklisted(c.Request().Context(), claims.JTI)
			if err != nil {
				if errors.Is(err, repository.ErrCacheUnavailable) {
					return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgAuthServiceUnavailable)})
				}
				return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgInternalError)})
			}
			if blacklisted {
				return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgTokenRevoked)})
			}

			c.Set(JWTContextKey, claims)
			return next(c)
		}
	}
}

// GetClaims extracts JWT claims from the echo context.
func GetClaims(c echo.Context) (*service.Claims, error) {
	claims, ok := c.Get(JWTContextKey).(*service.Claims)
	if !ok || claims == nil {
		return nil, errors.New("no JWT claims in context")
	}
	return claims, nil
}
