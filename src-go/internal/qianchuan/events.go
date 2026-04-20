package qianchuan

import (
	"time"

	"github.com/google/uuid"
)

// AuthExpiredPayload is the event payload emitted when a binding's token
// refresh fails terminally. Published on eventbus.EventAdsPlatformAuthExpired.
type AuthExpiredPayload struct {
	BindingID    uuid.UUID `json:"binding_id"`
	ProjectID    uuid.UUID `json:"project_id"`
	EmployeeID   *uuid.UUID `json:"employee_id,omitempty"`
	ProviderID   string    `json:"provider_id"`
	AdvertiserID string    `json:"advertiser_id"`
	Reason       string    `json:"reason"`
	DetectedAt   time.Time `json:"detected_at"`
}
