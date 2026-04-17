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
//	<platform-or-*>:<command-or-*>                 allow (legacy 2-segment form)
//	<tenant-or-*>:<platform-or-*>:<command-or-*>   allow (tenant-scoped form)
//	!<rule>                                         deny (takes precedence over allows)
//	(empty)                                         unrestricted — every command allowed
//
// Example: "feishu:/task,slack:/*,!slack:/tools" — legacy 2-segment form.
// Example: "acme:feishu:/task,*:slack:/*,!beta:slack:/tools" — tenant-scoped.
type CommandAllowlist struct {
	raw     string
	allows  []allowRule
	denies  []allowRule
	enabled bool
}

type allowRule struct {
	tenant   string // "" means "*"
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
		// Support both legacy `platform:command` and the new tenant-scoped
		// `tenant:platform:command`. A rule with three colon-separated
		// segments is treated as tenant-qualified; two segments leaves the
		// tenant dimension as "*" (wildcard).
		parts := strings.SplitN(entry, ":", 3)
		var rule allowRule
		switch len(parts) {
		case 2:
			rule = allowRule{
				platform: normalizeAllow(parts[0]),
				command:  normalizeAllow(parts[1]),
			}
		case 3:
			rule = allowRule{
				tenant:   normalizeAllow(parts[0]),
				platform: normalizeAllow(parts[1]),
				command:  normalizeAllow(parts[2]),
			}
		default:
			continue
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

// Permit evaluates the rules against the legacy (platform, command) key.
// Deny rules always take precedence. When no allow rule matches and the
// allowlist is enabled, the command is denied. This form is retained for
// call sites that do not yet have a resolved tenant.
func (al *CommandAllowlist) Permit(platform, command string) bool {
	return al.PermitTenant("", platform, command)
}

// PermitTenant is the tenant-scoped variant. An empty tenant id matches
// every rule's tenant wildcard; non-empty tenant ids must match explicitly
// or hit a `*` tenant rule.
func (al *CommandAllowlist) PermitTenant(tenantID, platform, command string) bool {
	if al == nil || !al.enabled {
		return true
	}
	for _, deny := range al.denies {
		if ruleMatches(deny, tenantID, platform, command) {
			return false
		}
	}
	for _, allow := range al.allows {
		if ruleMatches(allow, tenantID, platform, command) {
			return true
		}
	}
	return false
}

func ruleMatches(rule allowRule, tenantID, platform, command string) bool {
	if rule.tenant != "" && !strings.EqualFold(rule.tenant, tenantID) {
		return false
	}
	if rule.platform != "" && !strings.EqualFold(rule.platform, platform) {
		return false
	}
	if rule.command != "" && rule.command != command {
		return false
	}
	return true
}
