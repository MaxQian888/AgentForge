package middleware

import (
	"github.com/labstack/echo/v4"
	goI18n "github.com/nicksnyder/go-i18n/v2/i18n"
	appI18n "github.com/agentforge/marketplace/internal/i18n"
)

const LocalizerKey = "localizer"

// Locale is an Echo middleware that detects the request locale from
// the Accept-Language header and stores a go-i18n Localizer in the context.
func Locale() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			lang := c.Request().Header.Get("Accept-Language")
			localizer := appI18n.NewLocalizer(lang)
			c.Set(LocalizerKey, localizer)
			return next(c)
		}
	}
}

// GetLocalizer retrieves the Localizer stored by the Locale middleware.
func GetLocalizer(c echo.Context) *goI18n.Localizer {
	if l, ok := c.Get(LocalizerKey).(*goI18n.Localizer); ok {
		return l
	}
	return appI18n.NewLocalizer(appI18n.DefaultLocale)
}
