// Package i18n provides internationalization support for the marketplace service.
package i18n

import (
	"embed"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localeFS embed.FS

// Bundle is the global i18n message bundle.
var Bundle *i18n.Bundle

// DefaultLocale is the fallback locale used when no Accept-Language header is present.
const DefaultLocale = "en"

// Init initialises the global Bundle and loads all locale files.
func Init() {
	Bundle = i18n.NewBundle(language.English)
	Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	_, _ = Bundle.LoadMessageFileFS(localeFS, "locales/en.toml")
	_, _ = Bundle.LoadMessageFileFS(localeFS, "locales/zh-CN.toml")
}

// NewLocalizer creates a new Localizer for the given language tags.
func NewLocalizer(langs ...string) *i18n.Localizer {
	if Bundle == nil {
		Init()
	}
	return i18n.NewLocalizer(Bundle, normalizeLangs(langs)...)
}

// Localize returns the localized message for msgID, falling back to msgID on error.
func Localize(localizer *i18n.Localizer, msgID string) string {
	if Bundle == nil {
		Init()
	}
	if localizer == nil {
		localizer = NewLocalizer("en")
	}
	msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: msgID})
	if err != nil {
		return msgID
	}
	return msg
}

func normalizeLangs(langs []string) []string {
	filtered := make([]string, 0, len(langs))
	for _, lang := range langs {
		if trimmed := strings.TrimSpace(lang); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	if len(filtered) == 0 {
		return []string{DefaultLocale}
	}
	return filtered
}
