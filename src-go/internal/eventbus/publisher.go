// src-go/internal/eventbus/publisher.go
package eventbus

import "context"

// Publisher is the minimal interface services take so they can be tested with a fake bus.
type Publisher interface {
	Publish(ctx context.Context, e *Event) error
}
