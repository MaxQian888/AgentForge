package middleware

import (
	"github.com/labstack/echo/v4"

	applog "github.com/agentforge/server/internal/log"
)

const traceHeader = "X-Trace-ID"

// Trace returns an Echo middleware that resolves a correlation id for every request.
// Resolution order: X-Trace-ID header → X-Request-ID header → freshly generated.
// The resolved id is attached to the request context and echoed on the response.
func Trace() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			id := req.Header.Get(traceHeader)
			if id == "" {
				id = req.Header.Get(echo.HeaderXRequestID)
			}
			if id == "" {
				id = applog.NewTraceID()
			}
			ctx := applog.WithTrace(req.Context(), id)
			c.SetRequest(req.WithContext(ctx))
			c.Response().Header().Set(traceHeader, id)
			return next(c)
		}
	}
}
