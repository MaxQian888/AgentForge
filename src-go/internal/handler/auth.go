// Package handler implements HTTP request handlers for the API.
package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.authSvc.Register(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			return localizedError(c, http.StatusConflict, i18n.MsgEmailAlreadyExists)
		}
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			return localizedError(c, http.StatusServiceUnavailable, i18n.MsgAuthServiceUnavailable)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgRegistrationFailed)
	}

	return c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c echo.Context) error {
	req := new(model.LoginRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.authSvc.Login(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return localizedError(c, http.StatusUnauthorized, i18n.MsgInvalidCredentials)
		}
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			return localizedError(c, http.StatusServiceUnavailable, i18n.MsgAuthServiceUnavailable)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgLoginFailed)
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	req := new(model.RefreshRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.authSvc.Refresh(c.Request().Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return localizedError(c, http.StatusUnauthorized, i18n.MsgInvalidRefreshToken)
		}
		if errors.Is(err, repository.ErrCacheUnavailable) || errors.Is(err, repository.ErrDatabaseUnavailable) {
			return localizedError(c, http.StatusServiceUnavailable, i18n.MsgAuthServiceUnavailable)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgTokenRefreshFailed)
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Logout(c echo.Context) error {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}

	// Calculate remaining TTL of the access token
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining < 0 {
		remaining = 0
	}

	if err := h.authSvc.Logout(c.Request().Context(), claims.UserID, claims.JTI, remaining); err != nil {
		if errors.Is(err, repository.ErrCacheUnavailable) {
			return localizedError(c, http.StatusServiceUnavailable, i18n.MsgAuthServiceUnavailable)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgLogoutFailed)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "logged out successfully"})
}

func (h *AuthHandler) GetMe(c echo.Context) error {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}

	dto, err := h.authSvc.GetCurrentUser(c.Request().Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) || errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
		}
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			return localizedError(c, http.StatusServiceUnavailable, i18n.MsgAuthServiceUnavailable)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadUserProfile)
	}

	return c.JSON(http.StatusOK, dto)
}

func (h *AuthHandler) UpdateMe(c echo.Context) error {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	req := new(model.UpdateUserRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	dto, err := h.authSvc.UpdateProfile(c.Request().Context(), claims.UserID, req)
	if err != nil {
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			return localizedError(c, http.StatusServiceUnavailable, i18n.MsgAuthServiceUnavailable)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateProfile)
	}
	return c.JSON(http.StatusOK, dto)
}

func (h *AuthHandler) ChangePassword(c echo.Context) error {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	req := new(model.ChangePasswordRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	err = h.authSvc.ChangePassword(c.Request().Context(), claims.UserID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		if errors.Is(err, service.ErrCurrentPasswordIncorrect) {
			return localizedError(c, http.StatusBadRequest, i18n.MsgCurrentPasswordIncorrect)
		}
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			return localizedError(c, http.StatusServiceUnavailable, i18n.MsgAuthServiceUnavailable)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToChangePassword)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "password changed"})
}
