package mods

import (
	"context"
	"fmt"
	"strings"

	eb "github.com/agentforge/server/internal/eventbus"
)

type Auth struct{}

func NewAuth() *Auth {
	return &Auth{}
}

func (a *Auth) Name() string {
	return "core.auth"
}

func (a *Auth) Intercepts() []string {
	return []string{"*"}
}

func (a *Auth) Priority() int {
	return 20
}

func (a *Auth) Mode() eb.Mode {
	return eb.ModeGuard
}

func (a *Auth) Guard(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) error {
	if e.Source == "core" || strings.HasPrefix(e.Source, "plugin:") || strings.HasPrefix(e.Source, "agent:") {
		return nil
	}
	if strings.HasPrefix(e.Source, "user:") {
		claimed := strings.TrimPrefix(e.Source, "user:")
		ctxUser := eb.GetString(e, eb.MetaUserID)
		if ctxUser == "" || ctxUser == claimed {
			return nil
		}
		return fmt.Errorf("source user %q does not match context user %q", claimed, ctxUser)
	}
	return nil
}
