package core

import (
	"strings"
)

// CommandAllowlist decides whether a given (platform, command) pair is
// permitted to reach its handler. It is intended as a coarse-grained
// gate operators can flip via IM_COMMAND_ALLOWLIST while full RBAC stays
// at the backend. The allowlist MUST NOT substitute for backend RBAC; it
// exists to cut noise (e.g. during a limited pilot) without round-tripping.
//
// The rule grammar (comma-separated entries) is:
//
//	<platform-or-*>:<command-or-*>   allow
//	!<platform>:<command>            deny (takes precedence over allows)
//	(empty)                          unrestricted — every command allowed
//
// Example: "feishu:/task,feishu:/help,slack:/*,!slack:/tools"
type CommandAllowlist struct {
	raw     string
	allows  []allowRule
	denies  []allowRule
	enabled bool
}

type allowRule struct {
	platform string // "" means "*"
	command  string // "" means "*"
}

// NewCommandAllowlist parses the rule string and returns a matcher. An
// empty or whitespace-only string yields a matcher that admits everything.
func NewCommandAllowlist(raw string) *CommandAllowlist {
	raw = strings.TrimSpace(raw)
	al := &CommandAllowlist{raw: raw}
	if raw == "" {
		return al
	}
	al.enabled = true
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		isDeny := strings.HasPrefix(entry, "!")
		if isDeny {
			entry = strings.TrimPrefix(entry, "!")
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			continue
		}
		rule := allowRule{
			platform: normalizeAllow(parts[0]),
			command:  normalizeAllow(parts[1]),
		}
		if isDeny {
			al.denies = append(al.denies, rule)
		} else {
			al.allows = append(al.allows, rule)
		}
	}
	return al
}

func normalizeAllow(v string) string {
	v = strings.TrimSpace(v)
	// Accept both "*" and "/*" as the "match any command" token. "/*" is
	// how operators intuitively write "every slash command".
	if v == "*" || v == "/*" {
		return ""
	}
	return v
}

// Enabled reports whether the allowlist is active. When false, Permit
// always returns true.
func (al *CommandAllowlist) Enabled() bool {
	return al != nil && al.enabled
}

// Permit evaluates the rules. Deny rules always take precedence. When no
// allow rule matches and the allowlist is enabled, the command is denied.
func (al *CommandAllowlist) Permit(platform, command string) bool {
	if al == nil || !al.enabled {
		return true
	}
	for _, deny := range al.denies {
		if ruleMatches(deny, platform, command) {
			return false
		}
	}
	for _, allow := range al.allows {
		if ruleMatches(allow, platform, command) {
			return true
		}
	}
	return false
}

func ruleMatches(rule allowRule, platform, command string) bool {
	if rule.platform != "" && !strings.EqualFold(rule.platform, platform) {
		return false
	}
	if rule.command != "" && rule.command != command {
		return false
	}
	return true
}
