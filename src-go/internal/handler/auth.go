// Package handler implements HTTP request handlers for the API.
package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
)

type AuthHandler struct {
	authSvc      *service.AuthService
	jwtAccessTTL time.Duration
}

func NewAuthHandler(authSvc *service.AuthService, jwtAccessTTL time.Duration) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, jwtAccessTTL: jwtAccessTTL}
}

func (h *AuthHandler) Register(c echo.Context) error {
	req := new(model.RegisterRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.authSvc.Register(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: "email already exists"})
		}
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "authentication service unavailable"})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "registration failed"})
	}

	return c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c echo.Context) error {
	req := new(model.LoginRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.authSvc.Login(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "invalid email or password"})
		}
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "authentication service unavailable"})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "login failed"})
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	req := new(model.RefreshRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.authSvc.Refresh(c.Request().Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "invalid or expired refresh token"})
		}
		if errors.Is(err, repository.ErrCacheUnavailable) || errors.Is(err, repository.ErrDatabaseUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "authentication service unavailable"})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "token refresh failed"})
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Logout(c echo.Context) error {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "unauthorized"})
	}

	// Calculate remaining TTL of the access token
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining < 0 {
		remaining = 0
	}

	if err := h.authSvc.Logout(c.Request().Context(), claims.UserID, claims.JTI, remaining); err != nil {
		if errors.Is(err, repository.ErrCacheUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "authentication service unavailable"})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "logout failed"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "logged out successfully"})
}

func (h *AuthHandler) GetMe(c echo.Context) error {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "unauthorized"})
	}

	dto, err := h.authSvc.GetCurrentUser(c.Request().Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) || errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "unauthorized"})
		}
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "authentication service unavailable"})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to load user profile"})
	}

	return c.JSON(http.StatusOK, dto)
}
