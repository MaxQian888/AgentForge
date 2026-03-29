package service

import "strings"

func shouldIgnoreBridgeTaskID(taskID string) bool {
	return strings.HasPrefix(strings.TrimSpace(taskID), "__")
}
