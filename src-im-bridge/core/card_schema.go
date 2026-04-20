package core

import (
	"errors"
	"fmt"
)

// Provider-neutral card schema (spec §8). The IM Bridge accepts a
// ProviderNeutralCard via /im/send and dispatches it through the
// platform-specific renderer registered in card_renderer.go. The wire
// shape (the JSON keys + status enum strings) MUST stay stable across
// renderers and across the backend's outbound_dispatcher producer; the
// backend serializes this struct as the `card` field of /im/send.

// CardStatus mirrors spec §8 — "success/failed/running/pending/info".
type CardStatus string

const (
	CardStatusSuccess CardStatus = "success"
	CardStatusFailed  CardStatus = "failed"
	CardStatusRunning CardStatus = "running"
	CardStatusPending CardStatus = "pending"
	CardStatusInfo    CardStatus = "info"
)

// CardStyle is the visual emphasis hint for a button.
type CardStyle string

const (
	CardStylePrimary CardStyle = "primary"
	CardStyleDanger  CardStyle = "danger"
	CardStyleDefault CardStyle = "default"
)

// CardActionType discriminates URL vs callback buttons.
type CardActionType string

const (
	CardActionTypeURL      CardActionType = "url"
	CardActionTypeCallback CardActionType = "callback"
)

// CardField is a label-value pair displayed in the card body.
type CardField struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// CardAction is a flat-discriminated union: Type chooses URL vs Callback;
// the unused fields are omitempty so the wire shape matches spec §8.
type CardAction struct {
	ID               string         `json:"id"`
	Label            string         `json:"label"`
	Style            CardStyle      `json:"style,omitempty"`
	Type             CardActionType `json:"type"`
	URL              string         `json:"url,omitempty"`
	CorrelationToken string         `json:"correlation_token,omitempty"`
	Payload          map[string]any `json:"payload,omitempty"`
}

// Validate enforces the discriminated-union invariants the renderers rely on.
func (a CardAction) Validate() error {
	if a.ID == "" || a.Label == "" {
		return errors.New("card action: id and label are required")
	}
	switch a.Type {
	case CardActionTypeURL:
		if a.URL == "" {
			return errors.New("card action: url type requires url")
		}
	case CardActionTypeCallback:
		if a.CorrelationToken == "" {
			return errors.New("card action: callback type requires correlation_token")
		}
	default:
		return errors.New("card action: unknown type")
	}
	return nil
}

// ProviderNeutralCard is the platform-agnostic card the backend constructs
// and the IM Bridge translates to the active platform's card payload.
type ProviderNeutralCard struct {
	Title   string       `json:"title"`
	Status  CardStatus   `json:"status,omitempty"`
	Summary string       `json:"summary,omitempty"`
	Fields  []CardField  `json:"fields,omitempty"`
	Actions []CardAction `json:"actions,omitempty"`
	Footer  string       `json:"footer,omitempty"`
}

// Validate ensures the card has the minimum information required to render.
func (c ProviderNeutralCard) Validate() error {
	if c.Title == "" {
		return errors.New("card: title required")
	}
	for i, a := range c.Actions {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("card.actions[%d]: %w", i, err)
		}
	}
	return nil
}
