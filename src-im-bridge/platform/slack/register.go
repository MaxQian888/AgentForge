package slack

import (
	"fmt"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "slack",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"SLACK_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode == core.TransportModeLive {
				if env.Get("SLACK_BOT_TOKEN") == "" || env.Get("SLACK_APP_TOKEN") == "" {
					return fmt.Errorf("selected platform slack requires SLACK_BOT_TOKEN and SLACK_APP_TOKEN for live transport")
				}
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(env.Get("SLACK_BOT_TOKEN"), env.Get("SLACK_APP_TOKEN"))
		},
	})
}
