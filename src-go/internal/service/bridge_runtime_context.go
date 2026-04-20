package service

import (
	"strings"

	"github.com/agentforge/server/internal/model"
)

type BridgeRuntimeContextSnapshot struct {
	Runtime  string
	Provider string
	Model    string
	TeamID   string
	TeamRole string
}

func BridgeRuntimeContextFromRun(run *model.AgentRun) BridgeRuntimeContextSnapshot {
	if run == nil {
		return BridgeRuntimeContextSnapshot{}
	}
	snapshot := BridgeRuntimeContextSnapshot{
		Runtime:  strings.TrimSpace(run.Runtime),
		Provider: strings.TrimSpace(run.Provider),
		Model:    strings.TrimSpace(run.Model),
		TeamRole: strings.TrimSpace(run.TeamRole),
	}
	if run.TeamID != nil {
		snapshot.TeamID = strings.TrimSpace(run.TeamID.String())
	}
	return snapshot
}

func BridgeRuntimeContextFromStatus(status *BridgeStatusResponse) BridgeRuntimeContextSnapshot {
	if status == nil {
		return BridgeRuntimeContextSnapshot{}
	}
	return BridgeRuntimeContextSnapshot{
		Runtime:  strings.TrimSpace(status.Runtime),
		Provider: strings.TrimSpace(status.Provider),
		Model:    strings.TrimSpace(status.Model),
		TeamID:   strings.TrimSpace(status.TeamID),
		TeamRole: strings.TrimSpace(status.TeamRole),
	}
}

func DiffBridgeRuntimeContext(expected, actual BridgeRuntimeContextSnapshot) (field, expectedValue, actualValue string, ok bool) {
	comparisons := []struct {
		field    string
		expected string
		actual   string
	}{
		{field: "runtime", expected: expected.Runtime, actual: actual.Runtime},
		{field: "provider", expected: expected.Provider, actual: actual.Provider},
		{field: "model", expected: expected.Model, actual: actual.Model},
		{field: "team_id", expected: expected.TeamID, actual: actual.TeamID},
		{field: "team_role", expected: expected.TeamRole, actual: actual.TeamRole},
	}
	for _, comparison := range comparisons {
		if strings.TrimSpace(comparison.expected) == strings.TrimSpace(comparison.actual) {
			continue
		}
		if strings.TrimSpace(comparison.expected) == "" && strings.TrimSpace(comparison.actual) == "" {
			continue
		}
		return comparison.field, strings.TrimSpace(comparison.expected), strings.TrimSpace(comparison.actual), true
	}
	return "", "", "", false
}
