package wechat

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "wechat",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"WECHAT_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			if env.Get("WECHAT_APP_ID") == "" || env.Get("WECHAT_APP_SECRET") == "" {
				return fmt.Errorf("selected platform wechat requires WECHAT_APP_ID and WECHAT_APP_SECRET for live transport")
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			opts := make([]LiveOption, 0, 2)
			if p := strings.TrimSpace(env.Get("WECHAT_CALLBACK_PORT")); p != "" {
				opts = append(opts, WithCallbackPort(p))
			}
			if path := strings.TrimSpace(env.Get("WECHAT_CALLBACK_PATH")); path != "" {
				opts = append(opts, WithCallbackPath(path))
			}
			return NewLive(env.Get("WECHAT_APP_ID"), env.Get("WECHAT_APP_SECRET"), env.Get("WECHAT_CALLBACK_TOKEN"), opts...)
		},
	})
}
