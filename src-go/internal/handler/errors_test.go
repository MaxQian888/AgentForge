package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/model"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

func TestCustomHTTPErrorHandler_GenericError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler.CustomHTTPErrorHandler(errors.New("something went wrong"), c)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	var resp model.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Message != "internal server error" {
		t.Errorf("expected 'internal server error', got %q", resp.Message)
	}
}

func TestCustomHTTPErrorHandler_EchoHTTPError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler.CustomHTTPErrorHandler(echo.NewHTTPError(http.StatusNotFound, "not found"), c)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}

	var resp model.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Message != "not found" {
		t.Errorf("expected 'not found', got %q", resp.Message)
	}
}

func TestCustomHTTPErrorHandler_EchoHTTPError_NonStringMessage(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Echo sometimes has non-string messages
	handler.CustomHTTPErrorHandler(echo.NewHTTPError(http.StatusMethodNotAllowed, 405), c)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestCustomHTTPErrorHandler_CommittedResponse(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Commit the response first
	_ = c.String(http.StatusOK, "already committed")

	// Should not write anything else
	bodyBefore := rec.Body.String()
	handler.CustomHTTPErrorHandler(errors.New("should be ignored"), c)
	bodyAfter := rec.Body.String()

	if bodyBefore != bodyAfter {
		t.Error("expected no additional writes to committed response")
	}
}

func TestCustomHTTPErrorHandler_ValidationError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Trigger a real validation error
	v := validator.New()
	type testStruct struct {
		Email string `validate:"required,email"`
	}
	err := v.Struct(&testStruct{Email: ""})

	handler.CustomHTTPErrorHandler(err, c)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}

	var resp model.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Message == "" {
		t.Error("expected non-empty validation error message")
	}
}
