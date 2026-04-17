package core

import "strings"

// Unified reaction emoji codes used across providers. Each code maps to a
// provider-native emoji via ReactionEmojiMapFor.
const (
	ReactionAck        = "ack"
	ReactionRunning    = "running"
	ReactionDone       = "done"
	ReactionFailed     = "failed"
	ReactionThumbsUp   = "thumbs_up"
	ReactionThumbsDown = "thumbs_down"
	ReactionEyes       = "eyes"
	ReactionQuestion   = "question"
)

// DefaultReactionEmojiCodes returns the canonical unified emoji code set.
// Providers that advertise reaction support without a narrower set inherit
// this list.
func DefaultReactionEmojiCodes() []string {
	return []string{
		ReactionAck,
		ReactionRunning,
		ReactionDone,
		ReactionFailed,
		ReactionThumbsUp,
		ReactionThumbsDown,
		ReactionEyes,
		ReactionQuestion,
	}
}

// ReactionEmojiMap maps a unified code to a provider-native representation.
// The value is what the provider's API expects — a shortcode (Slack),
// emoji string (Telegram), or custom id (Feishu reaction_type).
type ReactionEmojiMap map[string]string

// ReactionEmojiMapFor returns the unified-code→provider-native mapping for
// a platform. Unknown platforms return nil; callers should then pass the
// unified code through as-is.
func ReactionEmojiMapFor(platform string) ReactionEmojiMap {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "slack":
		return ReactionEmojiMap{
			ReactionAck:        "eyes",
			ReactionRunning:    "gear",
			ReactionDone:       "white_check_mark",
			ReactionFailed:     "x",
			ReactionThumbsUp:   "+1",
			ReactionThumbsDown: "-1",
			ReactionEyes:       "eyes",
			ReactionQuestion:   "question",
		}
	case "discord":
		return ReactionEmojiMap{
			ReactionAck:        "\U0001F440",
			ReactionRunning:    "\u2699\uFE0F",
			ReactionDone:       "\u2705",
			ReactionFailed:     "\u274C",
			ReactionThumbsUp:   "\U0001F44D",
			ReactionThumbsDown: "\U0001F44E",
			ReactionEyes:       "\U0001F440",
			ReactionQuestion:   "\u2753",
		}
	case "feishu":
		return ReactionEmojiMap{
			ReactionAck:        "THUMBSUP",
			ReactionRunning:    "LOADING",
			ReactionDone:       "DONE",
			ReactionFailed:     "NO",
			ReactionThumbsUp:   "THUMBSUP",
			ReactionThumbsDown: "THUMBSDOWN",
			ReactionEyes:       "EYE",
			ReactionQuestion:   "QUESTION",
		}
	case "telegram":
		return ReactionEmojiMap{
			ReactionAck:        "\U0001F440",
			ReactionRunning:    "\u23F3",
			ReactionDone:       "\u2705",
			ReactionFailed:     "\u274C",
			ReactionThumbsUp:   "\U0001F44D",
			ReactionThumbsDown: "\U0001F44E",
			ReactionEyes:       "\U0001F440",
			ReactionQuestion:   "\u2753",
		}
	case "dingtalk":
		return ReactionEmojiMap{
			ReactionAck:        "\U0001F440",
			ReactionDone:       "\u2705",
			ReactionFailed:     "\u274C",
			ReactionThumbsUp:   "\U0001F44D",
			ReactionThumbsDown: "\U0001F44E",
		}
	default:
		return nil
	}
}

// ResolveReactionCode turns a provider-native emoji back into a unified code
// if one is known. Returns "" when no mapping exists (callers should then
// treat the raw value as the code).
func ResolveReactionCode(platform, raw string) string {
	mapping := ReactionEmojiMapFor(platform)
	if mapping == nil {
		return ""
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for code, native := range mapping {
		if strings.EqualFold(native, raw) {
			return code
		}
	}
	return ""
}

// NativeEmojiForCode turns a unified code into the provider-native value.
// If the mapping is missing, the unified code is returned verbatim so callers
// at least have something to send.
func NativeEmojiForCode(platform, code string) string {
	mapping := ReactionEmojiMapFor(platform)
	if mapping == nil {
		return code
	}
	if native, ok := mapping[strings.TrimSpace(code)]; ok {
		return native
	}
	return code
}
