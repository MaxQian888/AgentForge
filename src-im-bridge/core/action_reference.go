package core

import (
	"net/url"
	"sort"
	"strings"
)

func BuildActionReference(action string, entityID string, metadata map[string]string) string {
	action = strings.TrimSpace(action)
	entityID = strings.TrimSpace(entityID)
	if action == "" || entityID == "" {
		return ""
	}

	ref := "act:" + action + ":" + entityID
	if len(metadata) == 0 {
		return ref
	}

	values := url.Values{}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.TrimSpace(metadata[key])
		if strings.TrimSpace(key) == "" || value == "" {
			continue
		}
		values.Set(strings.TrimSpace(key), value)
	}
	if encoded := values.Encode(); encoded != "" {
		ref += "?" + encoded
	}
	return ref
}

// ParseActionReference parses the bridge-wide interactive action reference
// format `act:<action>:<entity-id>`.
func ParseActionReference(raw string) (action string, entityID string, ok bool) {
	action, entityID, _, ok = ParseActionReferenceWithMetadata(raw)
	return action, entityID, ok
}

func ParseActionReferenceWithMetadata(raw string) (action string, entityID string, metadata map[string]string, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.HasPrefix(trimmed, "link:") || !strings.HasPrefix(trimmed, "act:") {
		return "", "", nil, false
	}

	actionRef := strings.TrimSpace(strings.TrimPrefix(trimmed, "act:"))
	queryString := ""
	if idx := strings.Index(actionRef, "?"); idx >= 0 {
		queryString = strings.TrimSpace(actionRef[idx+1:])
		actionRef = strings.TrimSpace(actionRef[:idx])
	}
	parts := strings.SplitN(actionRef, ":", 2)
	if len(parts) != 2 {
		return "", "", nil, false
	}

	action = strings.TrimSpace(parts[0])
	entityID = strings.TrimSpace(parts[1])
	if action == "" || entityID == "" {
		return "", "", nil, false
	}
	if queryString != "" {
		values, err := url.ParseQuery(queryString)
		if err == nil {
			parsed := make(map[string]string, len(values))
			for key, entries := range values {
				if len(entries) == 0 {
					continue
				}
				value := strings.TrimSpace(entries[len(entries)-1])
				if strings.TrimSpace(key) == "" || value == "" {
					continue
				}
				parsed[strings.TrimSpace(key)] = value
			}
			if len(parsed) > 0 {
				metadata = parsed
			}
		}
	}
	return action, entityID, metadata, true
}

// Synthetic framework-level action names used when a card element click is
// not represented by an `act:<verb>:<entity>` reference. These are generated
// by the bridge-side normalizer and dispatched by the backend executor.
const (
	ActionNameReact       = "react"        // emoji reaction on a message
	ActionNameSelect      = "select"       // single-select click
	ActionNameMultiSelect = "multi_select" // multi-select click
	ActionNameDatePick    = "date_pick"    // date/time/datetime picker
	ActionNameOverflow    = "overflow"     // overflow ("…") menu click
	ActionNameToggle      = "toggle"       // checker element click
	ActionNameInputSubmit = "input_submit" // input element commit
	ActionNameFormSubmit  = "form_submit"  // form container submit
)
