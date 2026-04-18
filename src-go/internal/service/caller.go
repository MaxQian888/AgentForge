// Package service — caller.go declares the typed caller identity that gates
// agent-action service entrypoints (task dispatch, team run start, workflow
// execute, agent spawn, automation manual trigger).
//
// Two reasons to make this a typed parameter rather than reading from
// context.Context:
//
//   1. Compile-time enforcement. New call sites cannot forget the field —
//      the function signature requires it. This was the explicit ask in
//      the RBAC change Decision 3.
//   2. System-initiated paths must record a different identity (the
//      configuring user) than the caller. A typed Caller lets that flow
//      cleanly without relying on context smuggling.
//
// Validation policy is owned by the service that consumes the Caller.
// `Caller.Validate()` only checks structural invariants (e.g. SystemInitiated
// implies a configured-by user). Authorization happens against the RBAC
// matrix via middleware.Authorize when the service runs.
package service

import (
	"errors"

	"github.com/google/uuid"
)

// Caller identifies who initiated a service action. For human-initiated
// HTTP requests, UserID is the JWT claims subject and SystemInitiated is
// false. For scheduler/automation/IM-webhook-driven runs, SystemInitiated
// is true and ConfiguredByUserID identifies the human who last authorized
// the automation.
type Caller struct {
	UserID             uuid.UUID
	SystemInitiated    bool
	ConfiguredByUserID *uuid.UUID
	// RequestID is optional context for downstream audit emission. When
	// empty the audit event simply omits the field.
	RequestID string
}

// ErrCallerInvalid is returned by Validate when the structural contract
// is violated. Service entrypoints should treat this as a programmer
// error: the calling handler did not populate the Caller correctly.
var ErrCallerInvalid = errors.New("service: invalid caller identity")

// Validate enforces the structural invariants:
//   - human-initiated calls require a non-nil UserID
//   - system-initiated calls require a ConfiguredByUserID so audit can
//     trace the chain back to the human who set up the automation
func (c Caller) Validate() error {
	if c.SystemInitiated {
		if c.ConfiguredByUserID == nil {
			return errors.Join(ErrCallerInvalid, errors.New("system-initiated caller missing ConfiguredByUserID"))
		}
		return nil
	}
	if c.UserID == uuid.Nil {
		return errors.Join(ErrCallerInvalid, errors.New("human caller missing UserID"))
	}
	return nil
}

// EffectiveUserID returns the UserID of the human responsible for the
// action: the direct caller for human-initiated paths, or the
// configured-by user for system-initiated paths. Used by RBAC to pick
// which user's projectRole to evaluate.
func (c Caller) EffectiveUserID() uuid.UUID {
	if c.SystemInitiated && c.ConfiguredByUserID != nil {
		return *c.ConfiguredByUserID
	}
	return c.UserID
}
