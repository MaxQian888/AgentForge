package middleware

import (
	appI18n "github.com/agentforge/server/internal/i18n"
	"github.com/labstack/echo/v4"
	goI18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

const LocalizerKey = "localizer"

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

func GetLocalizer(c echo.Context) *goI18n.Localizer {
	if l, ok := c.Get(LocalizerKey).(*goI18n.Localizer); ok {
		return l
	}
	return appI18n.NewLocalizer(appI18n.DefaultLocale)
}
