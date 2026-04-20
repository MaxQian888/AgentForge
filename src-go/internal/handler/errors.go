package handler

import (
	"errors"
	"net/http"

	appI18n "github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// CustomHTTPErrorHandler is a centralized error handler registered on Echo.
// It acts as a safety net for unhandled errors and provides consistent error formatting.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	localizer := middleware.GetLocalizer(c)
	code := http.StatusInternalServerError
	message := appI18n.Localize(localizer, appI18n.MsgInternalError)

	// Validation errors from go-playground/validator
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		code = http.StatusUnprocessableEntity
		message = ve.Error()
	}

	// Echo HTTP errors (e.g. 404 from router, 405 method not allowed)
	var he *echo.HTTPError
	if errors.As(err, &he) {
		code = he.Code
		if m, ok := he.Message.(string); ok {
			message = m
		}
	}

	_ = c.JSON(code, model.ErrorResponse{Message: message, Code: code})
}
