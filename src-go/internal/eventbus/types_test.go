package eventbus

import "testing"

func TestEventOutboundDeliveryFailedConstantStable(t *testing.T) {
	if EventOutboundDeliveryFailed != "workflow.outbound_delivery.failed" {
		t.Fatalf("EventOutboundDeliveryFailed name must be stable for FE WS subscribers, got %q", EventOutboundDeliveryFailed)
	}
}
