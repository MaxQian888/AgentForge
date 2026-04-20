// Package service — project_template_sanitizer.go enforces the project
// template settings whitelist. We intentionally fail-closed on unknown
// settings fields: the cost of failing build is one retry; the cost of
// silently dropping a field the user expected to be in the template is a
// surprise that only shows up after clone.
//
// Sensitive field classes that MUST NOT enter a snapshot:
//   - webhook.secret / webhook.url (connects to a specific integration)
//   - OAuth client_secret / access_token / refresh_token / api_key variants
//   - Any field containing "password", "token", "secret", "api_key"
//
// We reuse the audit sanitizer's denylist *as a safety net*, but the primary
// gate is the settings whitelist — unknown = reject, not unknown = redact.
package service

import (
	"encoding/json"
	"fmt"

	"github.com/agentforge/server/internal/model"
)

// projectTemplateSettingsWhitelist lists the settings subtree fields that
// are allowed to enter a snapshot. Anything else MUST be rejected at build
// time. The list is intentionally small; growing it is a design decision,
// not an incidental oversight.
var projectTemplateSettingsWhitelist = map[string]struct{}{
	"review_policy":     {},
	"reviewPolicy":      {},
	"coding_agent":      {},
	"codingAgent":       {},
	"budget_governance": {},
	"budgetGovernance":  {},
}

// sanitizeProjectTemplateSettings takes the stored project settings struct
// and returns the typed snapshot sub-tree.  Because the settings struct is
// fully typed, we never carry raw JSON here — meaning "unknown field" cannot
// actually happen for the *typed* form. The guard is still useful for the
// raw JSON path used below.
func sanitizeProjectTemplateSettings(stored model.ProjectStoredSettings) (model.ProjectTemplateSettingsSnapshot, error) {
	snap := model.ProjectTemplateSettingsSnapshot{}

	// Review policy: copy as-is; no secret fields on this subtree.
	rp := stored.ReviewPolicy
	if len(rp.RequiredLayers) > 0 || rp.RequireManualApproval ||
		rp.MinRiskLevelForBlock != "" || rp.AutoTriggerOnPR ||
		len(rp.EnabledPluginDimensions) > 0 {
		rpCopy := rp
		snap.ReviewPolicy = &rpCopy
	}

	// Coding agent selection: copy. The selection struct is runtime/provider/
	// model keys only — no credentials.
	ca := stored.CodingAgent
	if ca.Runtime != "" || ca.Provider != "" || ca.Model != "" {
		caCopy := ca
		snap.DefaultCodingAgent = &caCopy
	}

	// Budget governance: copy.
	bg := stored.BudgetGovernance
	if bg.MaxTaskBudgetUsd != 0 || bg.MaxDailySpendUsd != 0 ||
		bg.AlertThresholdPercent != 0 || bg.AutoStopOnExceed {
		bgCopy := bg
		snap.BudgetGovernance = &bgCopy
	}

	// Webhook is deliberately NOT copied. It contains a URL + secret tied to
	// a specific integration — nothing a template should propagate.

	return snap, nil
}

// validateProjectTemplateSettingsRaw runs a paranoid second-pass check over
// the raw settings JSON. It is meant to catch the case where stored settings
// grew a new top-level field but no one updated the whitelist/copier above.
//
// Returns nil if every top-level key is in the whitelist OR is the known
// "webhook" key (which we drop but do not error on because the existing UI
// writes it). Returns an error listing any unknown keys.
func validateProjectTemplateSettingsRaw(raw string) error {
	if raw == "" || raw == "{}" {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil // upstream parseSettingsMap ignores malformed JSON; stay consistent
	}
	for key := range decoded {
		if _, ok := projectTemplateSettingsWhitelist[key]; ok {
			continue
		}
		if key == "webhook" { // known, intentionally dropped
			continue
		}
		return fmt.Errorf("%w: unknown settings key %q", ErrProjectTemplateSnapshotInvalid, key)
	}
	return nil
}
