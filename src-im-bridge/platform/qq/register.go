package qq

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "qq",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"QQ_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			if strings.TrimSpace(env.Get("QQ_ONEBOT_WS_URL")) == "" {
				return fmt.Errorf("selected platform qq requires QQ_ONEBOT_WS_URL for live transport")
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			return NewLive(env.Get("QQ_ONEBOT_WS_URL"), env.Get("QQ_ACCESS_TOKEN"))
		},
	})
}
