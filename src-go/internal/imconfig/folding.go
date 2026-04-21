// Package imconfig holds per-platform IM configuration defaults that do not
// change at runtime (as opposed to env-driven config in internal/config).
package imconfig

import "strings"

// FoldingMode controls how child-task events are forwarded to IM when the
// root task has an im_reply_target binding.
type FoldingMode string

const (
	// FoldingModeNested posts child-task events to IM using the root task's
	// reply target. Platforms that support threaded replies (feishu, slack,
	// telegram, discord, wecom) use this mode.
	FoldingModeNested FoldingMode = "nested"

	// FoldingModeFlat posts child-task events using the child's own reply
	// target if it has one, falling back to the root's target. Used as a
	// safe fallback for unknown platforms.
	FoldingModeFlat FoldingMode = "flat"

	// FoldingModeFrontendOnly suppresses IM posts for child-task events
	// entirely. Only the root task itself emits IM messages. Used for
	// platforms that do not support nested cards (qq, qqbot).
	FoldingModeFrontendOnly FoldingMode = "frontend_only"
)

// platformFoldingDefaults maps normalised platform names to their default
// folding mode. Platforms absent from this map fall back to FoldingModeFlat.
var platformFoldingDefaults = map[string]FoldingMode{
	"feishu":   FoldingModeNested,
	"slack":    FoldingModeNested,
	"telegram": FoldingModeNested,
	"discord":  FoldingModeNested,
	"wecom":    FoldingModeNested,
	"qq":       FoldingModeFrontendOnly,
	"qqbot":    FoldingModeFrontendOnly,
}

// FoldingModeFor returns the default FoldingMode for the given platform name.
// The lookup is case-insensitive. Unknown platforms return FoldingModeFlat.
func FoldingModeFor(platform string) FoldingMode {
	key := strings.ToLower(strings.TrimSpace(platform))
	if mode, ok := platformFoldingDefaults[key]; ok {
		return mode
	}
	return FoldingModeFlat
}
