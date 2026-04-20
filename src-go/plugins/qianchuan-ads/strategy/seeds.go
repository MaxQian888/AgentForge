package strategy

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/google/uuid"
)

//go:embed seeds/*.yaml
var seedFS embed.FS

// SeedRepo is the narrow seam SeedSystemStrategies consumes.
type SeedRepo interface {
	FindByProjectAndName(ctx context.Context, projectID *uuid.UUID, name string) ([]*QianchuanStrategy, error)
	Insert(ctx context.Context, s *QianchuanStrategy) error
}

// SeedSystemStrategies inserts every embedded seed YAML as a system row
// (project_id NULL, status=published) when no row of the same name exists.
// Idempotent: running twice does not produce duplicates.
//
// Errors short-circuit; a malformed seed YAML aborts startup so we never
// ship a half-seeded library.
func SeedSystemStrategies(ctx context.Context, repo SeedRepo) error {
	if repo == nil {
		return fmt.Errorf("seed repo is nil")
	}
	entries, err := fs.ReadDir(seedFS, "seeds")
	if err != nil {
		return fmt.Errorf("read embedded seeds: %w", err)
	}
	// Use a stable system actor UUID so audit trails remain deterministic.
	const systemUserID = "00000000-0000-0000-0000-000000000001"
	createdBy := uuid.MustParse(systemUserID)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := fs.ReadFile(seedFS, "seeds/"+e.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		strat, parsed, err := Parse(string(raw))
		if err != nil {
			return fmt.Errorf("parse seed %s: %w", e.Name(), err)
		}
		// Idempotency: skip if a row with this name + project=NULL exists.
		existing, err := repo.FindByProjectAndName(ctx, nil, strat.Name)
		if err != nil {
			return fmt.Errorf("lookup existing seed %s: %w", strat.Name, err)
		}
		if len(existing) > 0 {
			continue
		}
		encoded, err := json.Marshal(parsed)
		if err != nil {
			return fmt.Errorf("encode parsed seed %s: %w", strat.Name, err)
		}
		row := &QianchuanStrategy{
			ProjectID:   nil,
			Name:        strat.Name,
			Description: strat.Description,
			YAMLSource:  string(raw),
			ParsedSpec:  string(encoded),
			Version:     1,
			Status:      StatusPublished,
			CreatedBy:   createdBy,
		}
		if err := repo.Insert(ctx, row); err != nil {
			return fmt.Errorf("insert seed %s: %w", strat.Name, err)
		}
	}
	return nil
}
