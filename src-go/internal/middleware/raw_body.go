package middleware

import (
	"bytes"
	"io"

	"github.com/labstack/echo/v4"
)

// RawBodyKey is the echo.Context key where CaptureRawBody stores the
// original request body bytes. Downstream handlers retrieve it via
// c.Get(RawBodyKey).([]byte).
const RawBodyKey = "raw_body"

// CaptureRawBody reads the entire request body once, stashes it in c under
// RawBodyKey, and replaces c.Request().Body with a fresh reader so downstream
// handlers (Bind, validators) work normally. Required for HMAC-verified
// webhooks where the signed payload is the EXACT bytes on the wire.
func CaptureRawBody() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Body == nil {
				c.Set(RawBodyKey, []byte{})
				return next(c)
			}
			raw, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return echo.NewHTTPError(400, "read body: "+err.Error())
			}
			_ = c.Request().Body.Close()
			c.Request().Body = io.NopCloser(bytes.NewReader(raw))
			c.Set(RawBodyKey, raw)
			return next(c)
		}
	}
}
