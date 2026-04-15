// src-go/internal/eventbus/mod.go
package eventbus

import "context"

type Mode uint8

const (
	ModeGuard     Mode = 1
	ModeTransform Mode = 2
	ModeObserve   Mode = 3
)

type Mod interface {
	Name() string
	Intercepts() []string // glob-ish patterns, "*" matches all, prefix.* matches prefix.X
	Priority() int
	Mode() Mode
}

type GuardMod interface {
	Mod
	Guard(ctx context.Context, e *Event, pc *PipelineCtx) error
}

type TransformMod interface {
	Mod
	Transform(ctx context.Context, e *Event, pc *PipelineCtx) (*Event, error)
}

type ObserveMod interface {
	Mod
	Observe(ctx context.Context, e *Event, pc *PipelineCtx)
}

type PipelineCtx struct {
	NetworkID string
	Emits     []Event
	SpanID    string
	Attrs     map[string]any
}

func (pc *PipelineCtx) Emit(e Event) {
	pc.Emits = append(pc.Emits, e)
}

// MatchesType reports whether a glob pattern matches an event type.
// Supported forms: "*" | "prefix.*" | exact string.
func MatchesType(pattern, typ string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 2 && pattern[len(pattern)-2:] == ".*" {
		prefix := pattern[:len(pattern)-2]
		if typ == prefix {
			return true
		}
		if len(typ) > len(prefix) && typ[:len(prefix)+1] == prefix+"." {
			return true
		}
		return false
	}
	return pattern == typ
}
