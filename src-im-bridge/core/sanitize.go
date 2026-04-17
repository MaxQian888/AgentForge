package core

import (
	"regexp"
	"strings"
	"sync/atomic"
)

// defaultSanitizeMode is the process-wide sanitization mode. Set at startup
// by cmd/bridge/main.go from IM_SANITIZE_EGRESS. Callers within core read
// it through DefaultSanitizeMode() to decide whether to rewrite text.
var defaultSanitizeMode atomic.Value

func init() {
	defaultSanitizeMode.Store(SanitizeStrict)
}

// SetDefaultSanitizeMode installs the process-wide sanitization mode.
func SetDefaultSanitizeMode(mode SanitizeMode) {
	switch mode {
	case SanitizeOff, SanitizePermissive, SanitizeStrict:
		defaultSanitizeMode.Store(mode)
	}
}

// DefaultSanitizeMode returns the currently-configured mode.
func DefaultSanitizeMode() SanitizeMode {
	v := defaultSanitizeMode.Load()
	if v == nil {
		return SanitizeStrict
	}
	return v.(SanitizeMode)
}

// ParseSanitizeMode converts a user-supplied string into a SanitizeMode,
// accepting common case-insensitive spellings. Unknown values return the
// strict default so an operator misconfiguration fails safe.
func ParseSanitizeMode(raw string) SanitizeMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "off", "disable", "disabled", "none":
		return SanitizeOff
	case "permissive", "permit", "light":
		return SanitizePermissive
	case "strict", "":
		return SanitizeStrict
	default:
		return SanitizeStrict
	}
}

// SanitizeMode controls how aggressively outbound text is rewritten.
type SanitizeMode string

const (
	// SanitizeOff passes text through unchanged. Use only for debugging.
	SanitizeOff SanitizeMode = "off"
	// SanitizePermissive removes safety-invisible characters (zero-width,
	// stray control bytes) but keeps visible mentions intact.
	SanitizePermissive SanitizeMode = "permissive"
	// SanitizeStrict (default) additionally strips broadcast mentions and
	// enforces the provider's text-length limit.
	SanitizeStrict SanitizeMode = "strict"
)

// SanitizeWarning is a structured token added to delivery metadata so the
// audit stream can reconstruct what the sanitizer did.
type SanitizeWarning string

const (
	WarnBroadcastMentionStripped SanitizeWarning = "broadcast_mention_stripped"
	WarnTextTruncated            SanitizeWarning = "text_truncated"
	WarnTextSegmented            SanitizeWarning = "text_segmented"
	WarnZeroWidthStripped        SanitizeWarning = "zero_width_stripped"
	WarnControlCharStripped      SanitizeWarning = "control_char_stripped"
)

// SanitizeResult is returned from SanitizeEgress. Segments is non-empty iff
// the input had to be split to respect MaxTextLength and the provider
// profile allows segmentation. When Segments is empty, callers should use
// Text as the single sanitized payload.
type SanitizeResult struct {
	Text     string
	Segments []string
	Warnings []SanitizeWarning
}

// broadcastPatterns enumerates platform-visible broadcast tokens that
// strict mode replaces with the visible marker below. Patterns are ordered
// so longer tokens match first.
var broadcastPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<!channel>`),         // Slack
	regexp.MustCompile(`(?i)<!here>`),            // Slack
	regexp.MustCompile(`(?i)<!everyone>`),        // Slack (rare)
	regexp.MustCompile(`@everyone\b`),            // Discord/generic
	regexp.MustCompile(`@here\b`),                // Discord/generic
	regexp.MustCompile(`@all\b`),                 // Feishu/DingTalk/WeCom/QQ style
	regexp.MustCompile(`@channel\b`),             // Slack short form / Telegram channel
	regexp.MustCompile(`@(?:group|team|chat)\b`), // misc platform-specific
}

const broadcastReplacement = "[广播已屏蔽]"

// zeroWidth characters to strip unconditionally in permissive/strict modes.
var zeroWidthPattern = regexp.MustCompile(`[\x{200B}\x{200C}\x{200D}\x{FEFF}]`)

// SanitizeEgress applies the sanitizer to a single outbound text. The
// profile supplies MaxTextLength and SupportsSegments to drive length
// handling. mode selects aggressiveness.
func SanitizeEgress(profile RenderingProfile, mode SanitizeMode, text string) SanitizeResult {
	if mode == SanitizeOff || text == "" {
		return SanitizeResult{Text: text}
	}
	var warnings []SanitizeWarning

	// Zero-width characters: always stripped in permissive+strict.
	if zeroWidthPattern.MatchString(text) {
		text = zeroWidthPattern.ReplaceAllString(text, "")
		warnings = append(warnings, WarnZeroWidthStripped)
	}

	// Control characters (except \n \r \t): stripped in permissive+strict.
	stripped, removedControl := stripControlCharacters(text)
	if removedControl {
		text = stripped
		warnings = append(warnings, WarnControlCharStripped)
	}

	// Strict-mode extras.
	if mode == SanitizeStrict {
		text, warnings = stripBroadcastMentions(text, warnings)
		text, segments, warnings := applyLengthLimit(profile, text, warnings)
		return SanitizeResult{Text: text, Segments: segments, Warnings: warnings}
	}

	return SanitizeResult{Text: text, Warnings: warnings}
}

func stripBroadcastMentions(text string, warnings []SanitizeWarning) (string, []SanitizeWarning) {
	replaced := false
	for _, pat := range broadcastPatterns {
		if pat.MatchString(text) {
			text = pat.ReplaceAllString(text, broadcastReplacement)
			replaced = true
		}
	}
	if replaced {
		warnings = append(warnings, WarnBroadcastMentionStripped)
	}
	return text, warnings
}

func stripControlCharacters(text string) (string, bool) {
	var b strings.Builder
	removed := false
	for _, r := range text {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(r)
		case r < 0x20:
			removed = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String(), removed
}

func applyLengthLimit(profile RenderingProfile, text string, warnings []SanitizeWarning) (string, []string, []SanitizeWarning) {
	limit := profile.MaxTextLength
	if limit <= 0 || len(text) <= limit {
		return text, nil, warnings
	}
	if profile.SupportsSegments {
		segs := splitText(text, limit)
		warnings = append(warnings, WarnTextSegmented)
		return text, segs, warnings
	}
	truncated := truncateAtRuneBoundary(text, limit-len(truncationMarker)) + truncationMarker
	warnings = append(warnings, WarnTextTruncated)
	return truncated, nil, warnings
}

const truncationMarker = "…[已截断]"

// splitText slices text into chunks no larger than `limit` bytes. Splits
// prefer the nearest rune boundary before the hard limit so multi-byte
// characters never straddle chunks.
func splitText(text string, limit int) []string {
	if limit <= 0 {
		return []string{text}
	}
	var segs []string
	for len(text) > limit {
		cut := limit
		for cut > 0 && (text[cut]&0xC0) == 0x80 {
			cut--
		}
		if cut == 0 {
			cut = limit
		}
		segs = append(segs, text[:cut])
		text = text[cut:]
	}
	if len(text) > 0 {
		segs = append(segs, text)
	}
	return segs
}

func truncateAtRuneBoundary(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && (text[cut]&0xC0) == 0x80 {
		cut--
	}
	return text[:cut]
}
