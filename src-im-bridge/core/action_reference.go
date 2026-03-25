package core

import "strings"

// ParseActionReference parses the bridge-wide interactive action reference
// format `act:<action>:<entity-id>`.
func ParseActionReference(raw string) (action string, entityID string, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.HasPrefix(trimmed, "link:") || !strings.HasPrefix(trimmed, "act:") {
		return "", "", false
	}

	actionRef := strings.TrimSpace(strings.TrimPrefix(trimmed, "act:"))
	parts := strings.SplitN(actionRef, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	action = strings.TrimSpace(parts[0])
	entityID = strings.TrimSpace(parts[1])
	if action == "" || entityID == "" {
		return "", "", false
	}
	return action, entityID, true
}
