package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
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
		return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "unauthorized"})
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid user ID"})
	}

	unreadOnly := c.QueryParam("unread_only") == "true"
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	notifications, err := h.repo.ListByTarget(c.Request().Context(), userID, unreadOnly, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list notifications"})
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
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid notification ID"})
	}
	if err := h.repo.MarkRead(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to mark notification as read"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "notification marked as read"})
}
