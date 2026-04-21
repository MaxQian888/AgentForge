package wecom

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "wecom",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"WECOM_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			switch {
			case strings.TrimSpace(env.Get("WECOM_CORP_ID")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_CORP_ID for live transport")
			case strings.TrimSpace(env.Get("WECOM_AGENT_ID")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_AGENT_ID for live transport")
			case strings.TrimSpace(env.Get("WECOM_AGENT_SECRET")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_AGENT_SECRET for live transport")
			case strings.TrimSpace(env.Get("WECOM_CALLBACK_TOKEN")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_CALLBACK_TOKEN for live transport")
			case strings.TrimSpace(env.Get("WECOM_CALLBACK_PORT")) == "":
				return fmt.Errorf("selected platform wecom requires WECOM_CALLBACK_PORT for live transport")
			default:
				return nil
			}
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(
				env.Get("WECOM_CORP_ID"),
				env.Get("WECOM_AGENT_ID"),
				env.Get("WECOM_AGENT_SECRET"),
				env.Get("WECOM_CALLBACK_TOKEN"),
				env.Get("WECOM_CALLBACK_PORT"),
				env.Get("WECOM_CALLBACK_PATH"),
			)
		},
	})
}
