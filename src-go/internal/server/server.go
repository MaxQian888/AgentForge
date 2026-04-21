// Package server configures the Echo HTTP server with middleware and settings.
package server

import (
	"net/http"
	_ "net/http/pprof" // registers pprof handlers on http.DefaultServeMux
	"os"
	"time"

	"github.com/agentforge/server/internal/config"
	"github.com/agentforge/server/internal/handler"
	applog "github.com/agentforge/server/internal/log"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/repository"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	echolog "github.com/labstack/gommon/log"
	log "github.com/sirupsen/logrus"
)

// requireDebugToken gates a route behind a shared secret via the X-Debug-Token header.
// If DEBUG_TOKEN is unset, the route should not be mounted at all.
func requireDebugToken(token string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("X-Debug-Token") != token {
				return c.NoContent(http.StatusNotFound) // hide existence
			}
			return next(c)
		}
	}
}

type customValidator struct {
	validator *validator.Validate
}

func (cv *customValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

func New(cfg *config.Config, cache *repository.CacheRepository) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = false
	e.Validator = &customValidator{validator: validator.New()}
	e.HTTPErrorHandler = handler.CustomHTTPErrorHandler

	if cfg.Env == "production" {
		e.Logger.SetLevel(echolog.WARN)
	} else {
		e.Logger.SetLevel(echolog.DEBUG)
	}

	// Middleware stack (order matters)
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.RequestID())
	e.Use(appMiddleware.Trace()) // trace_id correlation — must precede request logger
	e.Use(echomiddleware.RequestLoggerWithConfig(echomiddleware.RequestLoggerConfig{
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogLatency:   true,
		LogRequestID: true,
		LogError:     true,
		HandleError:  true,
		LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
			fields := log.Fields{
				"method":     v.Method,
				"uri":        v.URI,
				"path":       c.Path(),
				"status":     v.Status,
				"latency_ms": v.Latency.Milliseconds(),
				"reqid":      v.RequestID,
				"trace_id":   applog.TraceID(c.Request().Context()),
				"remote_ip":  c.RealIP(),
			}
			if v.Error != nil {
				log.WithFields(fields).WithError(v.Error).Error("request")
			} else {
				log.WithFields(fields).Info("request")
			}
			return nil
		},
	}))
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:     cfg.AllowOrigins,
		AllowMethods:     []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS, echo.PATCH},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Request-ID", "X-Trace-ID", "Accept", "Accept-Language"},
		ExposeHeaders:    []string{"X-Request-ID", "X-Trace-ID"},
		AllowCredentials: true,
		MaxAge:           3600,
	}))
	e.Use(echomiddleware.SecureWithConfig(echomiddleware.SecureConfig{
		XSSProtection:      "1; mode=block",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "DENY",
	}))
	e.Use(appMiddleware.Locale())
	e.Use(echomiddleware.GzipWithConfig(echomiddleware.GzipConfig{Level: 5}))
	e.Use(echomiddleware.ContextTimeoutWithConfig(echomiddleware.ContextTimeoutConfig{
		Timeout: 30 * time.Second,
	}))

	// pprof — only mounted when DEBUG_TOKEN is set; requires X-Debug-Token header.
	if token := os.Getenv("DEBUG_TOKEN"); token != "" {
		pprofGroup := e.Group("/debug/pprof", requireDebugToken(token))
		pprofGroup.Any("/*", echo.WrapHandler(http.DefaultServeMux))
		log.WithField("path", "/debug/pprof/*").Info("pprof enabled (admin-gated)")
	}

	return e
}
