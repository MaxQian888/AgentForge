package eventbus

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityChannel Visibility = "channel"
	VisibilityDirect  Visibility = "direct"
	VisibilityModOnly Visibility = "mod_only"
)

type Event struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Source     string          `json:"source"`
	Target     string          `json:"target"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
	Timestamp  int64           `json:"timestamp"`
	Visibility Visibility      `json:"visibility"`
}

var typeRegexp = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func (e *Event) Validate() error {
	if e == nil {
		return fmt.Errorf("event: nil")
	}
	if e.ID == "" {
		return fmt.Errorf("event: id required")
	}
	if e.Type == "" {
		return fmt.Errorf("event: type required")
	}
	if !typeRegexp.MatchString(e.Type) {
		return fmt.Errorf("event: type %q violates {domain}.{entity}.{action} lowercase dot-notation", e.Type)
	}
	if e.Source == "" {
		return fmt.Errorf("event: source required")
	}
	if e.Target == "" {
		return fmt.Errorf("event: target required")
	}
	if e.Timestamp == 0 {
		return fmt.Errorf("event: timestamp required")
	}
	if e.Visibility == "" {
		e.Visibility = VisibilityChannel
	}
	return nil
}

func NewEvent(typ, source, target string) *Event {
	return &Event{
		ID:         uuid.New().String(),
		Type:       typ,
		Source:     source,
		Target:     target,
		Metadata:   map[string]any{},
		Timestamp:  time.Now().UnixMilli(),
		Visibility: VisibilityChannel,
	}
}
