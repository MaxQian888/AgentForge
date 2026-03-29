package handler

import (
	"github.com/labstack/echo/v4"
	appI18n "github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

func localizedError(c echo.Context, code int, msgID string) error {
	localizer := middleware.GetLocalizer(c)
	msg := appI18n.Localize(localizer, msgID)
	return c.JSON(code, model.ErrorResponse{Message: msg, Code: code})
}
