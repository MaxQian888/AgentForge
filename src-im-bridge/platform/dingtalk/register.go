package dingtalk

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "dingtalk",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"DINGTALK_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode == core.TransportModeLive {
				if env.Get("DINGTALK_APP_KEY") == "" || env.Get("DINGTALK_APP_SECRET") == "" {
					return fmt.Errorf("selected platform dingtalk requires DINGTALK_APP_KEY and DINGTALK_APP_SECRET for live transport")
				}
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			opts := make([]LiveOption, 0, 1)
			if tid := strings.TrimSpace(env.Get("DINGTALK_CARD_TEMPLATE_ID")); tid != "" {
				opts = append(opts, WithAdvancedCardTemplate(tid))
			}
			return NewLive(env.Get("DINGTALK_APP_KEY"), env.Get("DINGTALK_APP_SECRET"), opts...)
		},
	})
}
