package mods

import (
	"context"

	eb "github.com/agentforge/server/internal/eventbus"
)

type ChannelRouter struct{}

func NewChannelRouter() *ChannelRouter {
	return &ChannelRouter{}
}

func (c *ChannelRouter) Name() string         { return "core.channel-router" }
func (c *ChannelRouter) Intercepts() []string { return []string{"*"} }
func (c *ChannelRouter) Priority() int        { return 20 }
func (c *ChannelRouter) Mode() eb.Mode        { return eb.ModeTransform }

func (c *ChannelRouter) Transform(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) (*eb.Event, error) {
	channels := eb.GetChannels(e)
	seen := map[string]struct{}{}
	for _, ch := range channels {
		seen[ch] = struct{}{}
	}

	add := func(ch string) {
		if _, ok := seen[ch]; ok {
			return
		}
		seen[ch] = struct{}{}
		channels = append(channels, ch)
	}

	addr, err := eb.ParseAddress(e.Target)
	if err == nil {
		add("channel:" + addr.Scheme + ":" + addr.Name)
	}

	if pid := eb.GetString(e, eb.MetaProjectID); pid != "" {
		add(eb.MakeChannel("project", pid))
	}

	eb.SetChannels(e, channels)
	return e, nil
}
