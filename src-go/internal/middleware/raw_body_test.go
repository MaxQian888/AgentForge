package middleware_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/middleware"
	"github.com/labstack/echo/v4"
)

func TestCaptureRawBody_StashesAndRestores(t *testing.T) {
	e := echo.New()
	body := strings.NewReader(`{"hello":"world"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := middleware.CaptureRawBody()(func(c echo.Context) error {
		raw, ok := c.Get(middleware.RawBodyKey).([]byte)
		if !ok || string(raw) != `{"hello":"world"}` {
			return echo.NewHTTPError(500, "raw body not stashed")
		}
		// downstream handler can also re-read
		b, _ := io.ReadAll(c.Request().Body)
		if !bytes.Equal(b, []byte(`{"hello":"world"}`)) {
			return echo.NewHTTPError(500, "request body not restored")
		}
		return c.NoContent(204)
	})

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 204 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestCaptureRawBody_EmptyBody(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := middleware.CaptureRawBody()(func(c echo.Context) error {
		raw, _ := c.Get(middleware.RawBodyKey).([]byte)
		if len(raw) != 0 {
			return echo.NewHTTPError(500, "expected empty raw body")
		}
		return c.NoContent(204)
	})

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 204 {
		t.Fatalf("status %d", rec.Code)
	}
}
