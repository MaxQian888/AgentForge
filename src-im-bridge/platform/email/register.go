package email

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "email",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"EMAIL_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			switch {
			case strings.TrimSpace(env.Get("EMAIL_SMTP_HOST")) == "":
				return fmt.Errorf("selected platform email requires EMAIL_SMTP_HOST for live transport")
			case strings.TrimSpace(env.Get("EMAIL_FROM_ADDRESS")) == "":
				return fmt.Errorf("selected platform email requires EMAIL_FROM_ADDRESS for live transport")
			default:
				return nil
			}
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			opts := make([]LiveOption, 0, 1)
			if strings.EqualFold(strings.TrimSpace(env.Get("EMAIL_SMTP_TLS")), "false") {
				opts = append(opts, WithTLS(false))
			}
			return NewLive(
				env.Get("EMAIL_SMTP_HOST"),
				env.Get("EMAIL_SMTP_PORT"),
				env.Get("EMAIL_SMTP_USER"),
				env.Get("EMAIL_SMTP_PASS"),
				env.Get("EMAIL_FROM_ADDRESS"),
				opts...,
			)
		},
	})
}
