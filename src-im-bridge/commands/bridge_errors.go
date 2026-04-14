package commands

import "strings"

func describeBridgeFailure(err error) string {
	if err == nil {
		return "Bridge request failed"
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "delivery settlement"), strings.Contains(message, "delivery failed"):
		return "Delivery settlement failed"
	case strings.Contains(message, "runtime not ready"),
		strings.Contains(message, "missing executable"),
		strings.Contains(message, "missing_executable"),
		strings.Contains(message, "login required"),
		strings.Contains(message, "not authenticated"),
		strings.Contains(message, "not configured"):
		return "Runtime not ready"
	case strings.Contains(message, "bridge unavailable"),
		strings.Contains(message, "service unavailable"),
		strings.Contains(message, "api error 503"),
		strings.Contains(message, "api error 502"),
		strings.Contains(message, "bad gateway"):
		return "Bridge unavailable"
	default:
		return "Bridge request failed"
	}
}
