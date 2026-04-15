package mods

import (
	"context"
	"fmt"
	eb "github.com/react-go-quick-starter/server/internal/eventbus"
)

type Validate struct{}

func NewValidate() *Validate {
	return &Validate{}
}

func (v *Validate) Name() string {
	return "core.validate"
}

func (v *Validate) Intercepts() []string {
	return []string{"*"}
}

func (v *Validate) Priority() int {
	return 10
}

func (v *Validate) Mode() eb.Mode {
	return eb.ModeGuard
}

func (v *Validate) Guard(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) error {
	if _, err := eb.ParseAddress(e.Source); err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}
	if _, err := eb.ParseAddress(e.Target); err != nil {
		return fmt.Errorf("invalid target: %w", err)
	}
	if raw, ok := e.Metadata[eb.MetaChannels]; ok {
		switch raw.(type) {
		case []string, []any, nil:
		default:
			return fmt.Errorf("metadata.channels must be []string, got %T", raw)
		}
	}
	return nil
}
