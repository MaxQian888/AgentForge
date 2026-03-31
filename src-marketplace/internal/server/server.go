// Package server configures the Echo HTTP server with middleware and settings.
package server

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	echolog "github.com/labstack/gommon/log"
	log "github.com/sirupsen/logrus"

	"github.com/agentforge/marketplace/internal/config"
	"github.com/agentforge/marketplace/internal/handler"
	appMiddleware "github.com/agentforge/marketplace/internal/middleware"
)

type customValidator struct {
	validator *validator.Validate
}

func (cv *customValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

// New creates and configures a new Echo instance.
func New(cfg *config.Config) *echo.Echo {
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
				"status":     v.Status,
				"latency_ms": v.Latency.Milliseconds(),
				"reqid":      v.RequestID,
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
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Request-ID", "Accept", "Accept-Language"},
		ExposeHeaders:    []string{"X-Request-ID", "X-Content-Digest"},
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

	return e
}
