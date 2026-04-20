// Package service — project_template_upgrades.go holds the snapshot version
// upgrade registry. When a future change bumps
// model.CurrentProjectTemplateSnapshotVersion, add an upgrade function here
// keyed by the prior version; ApplySnapshot runs the chain in order.
//
// Design note: upgrades are one-way. An older client cannot downgrade a
// snapshot from a newer version; it will see an error and the user will be
// prompted to recreate the template. We accept that trade-off because the
// alternative — silently dropping fields — turns templates into lossy data.
package service

import (
	"fmt"

	"github.com/agentforge/server/internal/model"
)

// projectTemplateSnapshotUpgrade is the function shape. Each entry must
// preserve data integrity across a single-version hop.
type projectTemplateSnapshotUpgrade func(model.ProjectTemplateSnapshot) (model.ProjectTemplateSnapshot, error)

// projectTemplateSnapshotUpgrades maps "from version" → upgrade fn that
// produces a "from version + 1" snapshot. Empty for now because we only have
// version 1; kept as a typed map so the code below doesn't special-case
// "no upgrades yet".
var projectTemplateSnapshotUpgrades = map[int]projectTemplateSnapshotUpgrade{
	// Example for future: 1: upgradeFromV1ToV2,
}

// upgradeProjectTemplateSnapshot steps a snapshot forward to the current
// version by iteratively applying registered upgraders.
func upgradeProjectTemplateSnapshot(s model.ProjectTemplateSnapshot) (model.ProjectTemplateSnapshot, error) {
	for s.Version < model.CurrentProjectTemplateSnapshotVersion {
		fn, ok := projectTemplateSnapshotUpgrades[s.Version]
		if !ok {
			return s, fmt.Errorf("no upgrade registered from snapshot version %d", s.Version)
		}
		next, err := fn(s)
		if err != nil {
			return s, fmt.Errorf("upgrade snapshot v%d→v%d: %w", s.Version, s.Version+1, err)
		}
		if next.Version <= s.Version {
			return s, fmt.Errorf("upgrade from v%d produced version %d — upgrade fn must advance version", s.Version, next.Version)
		}
		s = next
	}
	if s.Version > model.CurrentProjectTemplateSnapshotVersion {
		return s, fmt.Errorf("snapshot version %d is newer than this server supports (max %d); recreate template", s.Version, model.CurrentProjectTemplateSnapshotVersion)
	}
	return s, nil
}
