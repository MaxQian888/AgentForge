package handler

import (
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"

	"github.com/agentforge/marketplace/internal/model"
)

// CustomHTTPErrorHandler is a centralized error handler registered on Echo.
// It provides consistent error formatting for all unhandled errors.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	message := "Internal server error"

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

	_ = c.JSON(code, model.ErrorResponse{Message: message})
}
