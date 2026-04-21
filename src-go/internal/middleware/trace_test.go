package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	applog "github.com/agentforge/server/internal/log"
	mw "github.com/agentforge/server/internal/middleware"
)

func TestTrace_UsesInboundHeader(t *testing.T) {
	e := echo.New()
	e.Use(mw.Trace())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, applog.TraceID(c.Request().Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "tr_inbound0000000000000000")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Body.String() != "tr_inbound0000000000000000" {
		t.Fatalf("want inbound trace, got %q", rec.Body.String())
	}
	if got := rec.Header().Get("X-Trace-ID"); got != "tr_inbound0000000000000000" {
		t.Fatalf("want echo response header, got %q", got)
	}
}

func TestTrace_GeneratesWhenMissing(t *testing.T) {
	e := echo.New()
	e.Use(mw.Trace())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, applog.TraceID(c.Request().Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.HasPrefix(body, "tr_") || len(body) != 27 {
		t.Fatalf("want generated trace_id, got %q", body)
	}
	if rec.Header().Get("X-Trace-ID") != body {
		t.Fatalf("response header must match ctx trace")
	}
}

func TestTrace_FallsBackToRequestID(t *testing.T) {
	e := echo.New()
	e.Use(mw.Trace())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, applog.TraceID(c.Request().Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req-abc")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Body.String() != "req-abc" {
		t.Fatalf("want fallback to X-Request-ID, got %q", rec.Body.String())
	}
}
