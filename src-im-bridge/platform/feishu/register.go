package feishu

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "feishu",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"FEISHU_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode == core.TransportModeLive {
				if env.Get("FEISHU_APP_ID") == "" || env.Get("FEISHU_APP_SECRET") == "" {
					return fmt.Errorf("selected platform feishu requires FEISHU_APP_ID and FEISHU_APP_SECRET for live transport")
				}
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			opts := make([]LiveOption, 0, 1)
			if strings.TrimSpace(env.Get("FEISHU_VERIFICATION_TOKEN")) != "" {
				opts = append(opts, WithCardCallbackWebhook(
					env.Get("FEISHU_VERIFICATION_TOKEN"),
					env.Get("FEISHU_EVENT_ENCRYPT_KEY"),
					env.Get("FEISHU_CALLBACK_PATH"),
				))
			}
			return NewLive(env.Get("FEISHU_APP_ID"), env.Get("FEISHU_APP_SECRET"), opts...)
		},
	})
}
