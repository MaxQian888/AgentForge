// src-go/internal/eventbus/pipeline.go
package eventbus

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const observerTimeout = 5 * time.Second

type Pipeline struct {
	guards     []GuardMod
	transforms []TransformMod
	observes   []ObserveMod
}

func NewPipeline(mods []Mod) *Pipeline {
	p := &Pipeline{}
	for _, m := range mods {
		switch m.Mode() {
		case ModeGuard:
			if g, ok := m.(GuardMod); ok {
				p.guards = append(p.guards, g)
			}
		case ModeTransform:
			if t, ok := m.(TransformMod); ok {
				p.transforms = append(p.transforms, t)
			}
		case ModeObserve:
			if o, ok := m.(ObserveMod); ok {
				p.observes = append(p.observes, o)
			}
		}
	}
	sort.SliceStable(p.guards, func(i, j int) bool { return p.guards[i].Priority() < p.guards[j].Priority() })
	sort.SliceStable(p.transforms, func(i, j int) bool { return p.transforms[i].Priority() < p.transforms[j].Priority() })
	sort.SliceStable(p.observes, func(i, j int) bool { return p.observes[i].Priority() < p.observes[j].Priority() })
	return p
}

func (p *Pipeline) Process(ctx context.Context, e *Event, pc *PipelineCtx) (*Event, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	for _, g := range p.guards {
		if !intercepts(g, e.Type) {
			continue
		}
		if err := g.Guard(ctx, e, pc); err != nil {
			return nil, fmt.Errorf("guard %q rejected: %w", g.Name(), err)
		}
	}
	cur := e
	for _, t := range p.transforms {
		if !intercepts(t, cur.Type) {
			continue
		}
		out, err := t.Transform(ctx, cur, pc)
		if err != nil {
			return nil, fmt.Errorf("transform %q failed: %w", t.Name(), err)
		}
		if out != nil {
			cur = out
		}
	}
	// Fan out observers in parallel; cap with deadline; recover panics.
	var wg sync.WaitGroup
	for _, o := range p.observes {
		if !intercepts(o, cur.Type) {
			continue
		}
		wg.Add(1)
		go func(obs ObserveMod) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(log.Fields{"mod": obs.Name(), "panic": r}).
						Error("eventbus: observer panic")
				}
			}()
			cctx, cancel := context.WithTimeout(ctx, observerTimeout)
			defer cancel()
			obs.Observe(cctx, cur, pc)
		}(o)
	}
	wg.Wait()
	return cur, nil
}

func intercepts(m Mod, typ string) bool {
	for _, pat := range m.Intercepts() {
		if MatchesType(pat, typ) {
			return true
		}
	}
	return false
}
