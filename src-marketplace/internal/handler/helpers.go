package handler

import (
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// localizedError returns a JSON error response with the given message.
func localizedError(c echo.Context, statusCode int, message string) error {
	return c.JSON(statusCode, map[string]string{"message": message})
}

// claimsUserID extracts the user UUID from JWT claims stored in the Echo context.
func claimsUserID(c echo.Context) (uuid.UUID, error) {
	token, ok := c.Get("user").(*jwt.Token)
	if !ok {
		return uuid.Nil, echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	sub, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	return uuid.Parse(sub)
}

// claimsUserName extracts the user name (or email) from JWT claims.
func claimsUserName(c echo.Context) string {
	token, ok := c.Get("user").(*jwt.Token)
	if !ok {
		return ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	name, _ := claims["name"].(string)
	if name == "" {
		name, _ = claims["email"].(string)
	}
	return name
}
