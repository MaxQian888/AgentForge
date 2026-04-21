package telegram

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "telegram",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"TELEGRAM_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			if env.Get("TELEGRAM_BOT_TOKEN") == "" {
				return fmt.Errorf("selected platform telegram requires TELEGRAM_BOT_TOKEN for live transport")
			}
			return telegramValidateUpdateMode(env.Get("TELEGRAM_UPDATE_MODE"), env.Get("TELEGRAM_WEBHOOK_URL"))
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(env.Get("TELEGRAM_BOT_TOKEN"))
		},
	})
}

// telegramValidateUpdateMode is lifted verbatim from the telegramValidateConfig
// helper in cmd/bridge/platform_registry.go.
func telegramValidateUpdateMode(updateMode, webhookURL string) error {
	normalized := strings.ToLower(strings.TrimSpace(updateMode))
	if normalized == "" {
		normalized = "longpoll"
	}
	if normalized != "longpoll" {
		return fmt.Errorf("telegram live transport currently supports only longpoll update mode")
	}
	if strings.TrimSpace(webhookURL) != "" {
		return fmt.Errorf("telegram long polling cannot be combined with webhook configuration")
	}
	return nil
}
