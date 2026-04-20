package mods

import (
	"context"

	eb "github.com/agentforge/server/internal/eventbus"
)

type Enrich struct{}

func NewEnrich() *Enrich {
	return &Enrich{}
}

func (e *Enrich) Name() string         { return "core.enrich" }
func (e *Enrich) Intercepts() []string { return []string{"*"} }
func (e *Enrich) Priority() int        { return 10 }
func (e *Enrich) Mode() eb.Mode        { return eb.ModeTransform }

func (e *Enrich) Transform(ctx context.Context, ev *eb.Event, pc *eb.PipelineCtx) (*eb.Event, error) {
	if pc.SpanID != "" && eb.GetString(ev, eb.MetaSpanID) == "" {
		eb.SetString(ev, eb.MetaSpanID, pc.SpanID)
	}
	return ev, nil
}
