package eventbus

import (
	"context"
	"encoding/json"
)

// PublishLegacy is a thin adapter matching the pre-eventbus "type + projectID
// + payload" shape used by every legacy hub.BroadcastEvent call-site. It
// keeps the migration mechanical: services carry a Publisher (the *Bus) and
// replace their old broadcast with a single call here.
//
// Behaviour:
//   - Channel defaults to "project:<projectID>" when projectID is set,
//     falling back to "system:broadcast" when empty.
//   - Target mirrors the channel scope so core.channel-router + ws-fanout
//     deliver it to the right subscribers without bespoke metadata.
//   - Payload is JSON-marshalled; a marshal failure silently drops (parity
//     with the old Hub behaviour, which also best-efforted).
//
// Errors from Publish are returned so callers can optionally log; existing
// call-sites which discarded the error (the common case) can continue to
// discard it.
func PublishLegacy(ctx context.Context, p Publisher, eventType, projectID string, payload any) error {
	if p == nil {
		return nil
	}
	var target string
	var channel string
	if projectID != "" {
		target = MakeProject(projectID)
		channel = "project:" + projectID
	} else {
		target = "system:broadcast"
		channel = "system:broadcast"
	}
	e := NewEvent(eventType, "core", target)
	if projectID != "" {
		SetString(e, MetaProjectID, projectID)
	}
	SetChannels(e, []string{channel})

	if payload != nil {
		data, err := json.Marshal(payload)
		if err == nil {
			e.Payload = data
		}
	}
	return p.Publish(ctx, e)
}
