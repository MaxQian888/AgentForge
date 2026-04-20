package qchandler

import (
	"github.com/labstack/echo/v4"
	appI18n "github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

// localizedError mirrors the unexported helper of the same name in
// internal/handler so the plugin's handlers keep identical behavior
// without taking a dependency on a private core helper. Keep the two
// implementations in lockstep or expose a shared helper in a future
// refactor.
func localizedError(c echo.Context, code int, msgID string) error {
	localizer := middleware.GetLocalizer(c)
	msg := appI18n.Localize(localizer, msgID)
	return c.JSON(code, model.ErrorResponse{Message: msg, Code: code})
}
