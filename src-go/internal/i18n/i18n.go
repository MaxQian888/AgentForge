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

var Bundle *i18n.Bundle

const DefaultLocale = "en"

func Init() {
	Bundle = i18n.NewBundle(language.English)
	Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	Bundle.LoadMessageFileFS(localeFS, "locales/en.toml")
	Bundle.LoadMessageFileFS(localeFS, "locales/zh-CN.toml")
}

func NewLocalizer(langs ...string) *i18n.Localizer {
	if Bundle == nil {
		Init()
	}
	return i18n.NewLocalizer(Bundle, normalizeLangs(langs)...)
}

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
