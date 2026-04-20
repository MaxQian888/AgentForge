package handler

import (
	appI18n "github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/labstack/echo/v4"
)

func localizedError(c echo.Context, code int, msgID string) error {
	localizer := middleware.GetLocalizer(c)
	msg := appI18n.Localize(localizer, msgID)
	return c.JSON(code, model.ErrorResponse{Message: msg, Code: code})
}
