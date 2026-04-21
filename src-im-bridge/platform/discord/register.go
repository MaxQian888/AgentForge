package discord

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "discord",
		Metadata:                liveMetadata,
		SupportedTransportModes: []string{core.TransportModeStub, core.TransportModeLive},
		EnvPrefixes:             []string{"DISCORD_"},
		ValidateConfig: func(env core.ProviderEnv, mode string) error {
			if mode != core.TransportModeLive {
				return nil
			}
			if env.Get("DISCORD_APP_ID") == "" || env.Get("DISCORD_BOT_TOKEN") == "" || env.Get("DISCORD_PUBLIC_KEY") == "" {
				return fmt.Errorf("selected platform discord requires DISCORD_APP_ID, DISCORD_BOT_TOKEN, and DISCORD_PUBLIC_KEY for live transport")
			}
			if env.Get("DISCORD_INTERACTIONS_PORT") == "" {
				return fmt.Errorf("selected platform discord requires DISCORD_INTERACTIONS_PORT for live transport")
			}
			return nil
		},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return NewStub(env.TestPort()), nil
		},
		NewLive: func(env core.ProviderEnv) (core.Platform, error) {
			opts := make([]LiveOption, 0, 1)
			if gid := strings.TrimSpace(env.Get("DISCORD_COMMAND_GUILD_ID")); gid != "" {
				opts = append(opts, WithCommandGuildID(gid))
			}
			return NewLive(
				env.Get("DISCORD_APP_ID"),
				env.Get("DISCORD_BOT_TOKEN"),
				env.Get("DISCORD_PUBLIC_KEY"),
				env.Get("DISCORD_INTERACTIONS_PORT"),
				opts...,
			)
		},
	})
}
