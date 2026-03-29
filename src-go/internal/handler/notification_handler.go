package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type NotificationHandler struct {
	repo *repository.NotificationRepository
}

func NewNotificationHandler(repo *repository.NotificationRepository) *NotificationHandler {
	return &NotificationHandler{repo: repo}
}

func (h *NotificationHandler) List(c echo.Context) error {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidUserID)
	}

	unreadOnly := c.QueryParam("unread_only") == "true"
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	notifications, err := h.repo.ListByTarget(c.Request().Context(), userID, unreadOnly, limit)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListNotifications)
	}

	dtos := make([]model.NotificationDTO, 0, len(notifications))
	for _, n := range notifications {
		dtos = append(dtos, n.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *NotificationHandler) MarkRead(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidNotificationID)
	}
	if err := h.repo.MarkRead(c.Request().Context(), id); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToMarkNotificationRead)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "notification marked as read"})
}

func (h *NotificationHandler) MarkAllRead(c echo.Context) error {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidUserID)
	}

	if err := h.repo.MarkAllRead(c.Request().Context(), userID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToMarkAllRead)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "notifications marked as read"})
}
