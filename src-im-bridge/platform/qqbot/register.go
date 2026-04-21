package qqbot

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "qqbot",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"QQBOT_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			switch {
			case strings.TrimSpace(env.Get("QQBOT_APP_ID")) == "":
				return fmt.Errorf("selected platform qqbot requires QQBOT_APP_ID for live transport")
			case strings.TrimSpace(env.Get("QQBOT_APP_SECRET")) == "":
				return fmt.Errorf("selected platform qqbot requires QQBOT_APP_SECRET for live transport")
			case strings.TrimSpace(env.Get("QQBOT_CALLBACK_PORT")) == "":
				return fmt.Errorf("selected platform qqbot requires QQBOT_CALLBACK_PORT for live transport")
			default:
				return nil
			}
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(
				env.Get("QQBOT_APP_ID"),
				env.Get("QQBOT_APP_SECRET"),
				env.Get("QQBOT_CALLBACK_PORT"),
				env.Get("QQBOT_CALLBACK_PATH"),
				WithAPIBase(env.Get("QQBOT_API_BASE")),
				WithTokenBase(env.Get("QQBOT_TOKEN_BASE")),
			)
		},
	})
}
