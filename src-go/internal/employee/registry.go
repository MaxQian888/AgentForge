package employee

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"gopkg.in/yaml.v3"
)

// Manifest is the YAML shape for a seeded employee template.
type Manifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		ID          string `yaml:"id"`
		Name        string `yaml:"name"`
		DisplayName string `yaml:"displayName"`
	} `yaml:"metadata"`
	RoleID       string         `yaml:"role_id"`
	RuntimePrefs map[string]any `yaml:"runtime_prefs"`
	Config       map[string]any `yaml:"config"`
	ExtraSkills  []struct {
		Path     string `yaml:"path"`
		AutoLoad bool   `yaml:"auto_load"`
	} `yaml:"extra_skills"`
}

// SeedReport summarises what happened during a SeedFromDir run.
type SeedReport struct {
	Upserted int
	Skipped  int
	Errors   []error
}

// Registry seeds YAML-defined Employees into projects via the Service layer.
type Registry struct {
	svc *Service
}

// NewRegistry constructs a Registry backed by the given Service.
func NewRegistry(svc *Service) *Registry {
	return &Registry{svc: svc}
}

// SeedFromDir loads every *.yaml / *.yml file in dir, parses it as a Manifest
// (requires Kind=="Employee"), and for each listed project upserts an
// Employee with Name=Manifest.Metadata.ID. Re-running is idempotent: an
// existing Employee with the same (project_id, name) is treated as Skipped.
// Errors per-file / per-project are collected in SeedReport.Errors but don't
// abort the overall seed pass.
func (r *Registry) SeedFromDir(ctx context.Context, dir string, projectIDs []uuid.UUID) (SeedReport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return SeedReport{}, fmt.Errorf("read employee manifest dir %s: %w", dir, err)
	}

	var report SeedReport

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		manifest, err := parseManifestFile(path)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("file %s: %w", entry.Name(), err))
			continue
		}

		if err := validateManifest(manifest); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("file %s: %w", entry.Name(), err))
			continue
		}

		for _, projectID := range projectIDs {
			upserted, err := r.upsertForProject(ctx, manifest, projectID)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("file %s project %s: %w", entry.Name(), projectID, err))
				continue
			}
			if upserted {
				report.Upserted++
			} else {
				report.Skipped++
			}
		}
	}

	return report, nil
}

// upsertForProject checks whether an Employee with Name==manifest.Metadata.ID
// already exists in the project; if not, creates one. Returns true when a new
// Employee was created.
func (r *Registry) upsertForProject(ctx context.Context, m *Manifest, projectID uuid.UUID) (bool, error) {
	existing, err := r.svc.ListByProject(ctx, projectID, repository.EmployeeFilter{})
	if err != nil {
		return false, fmt.Errorf("list employees: %w", err)
	}

	for _, e := range existing {
		if e.Name == m.Metadata.ID {
			return false, nil // already seeded — idempotent skip
		}
	}

	skills := make([]model.EmployeeSkill, 0, len(m.ExtraSkills))
	for _, sk := range m.ExtraSkills {
		skills = append(skills, model.EmployeeSkill{
			SkillPath: sk.Path,
			AutoLoad:  sk.AutoLoad,
		})
	}

	_, err = r.svc.Create(ctx, CreateInput{
		ProjectID:    projectID,
		Name:         m.Metadata.ID,
		DisplayName:  firstNonEmpty(m.Metadata.DisplayName, m.Metadata.Name),
		RoleID:       m.RoleID,
		RuntimePrefs: mustJSON(m.RuntimePrefs),
		Config:       mustJSON(m.Config),
		Skills:       skills,
	})
	if err != nil {
		if errors.Is(err, ErrEmployeeNameExists) {
			return false, nil // race-safe idempotent skip
		}
		return false, fmt.Errorf("create employee: %w", err)
	}
	return true, nil
}

// parseManifestFile reads and unmarshals a single YAML manifest file.
func parseManifestFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	return &m, nil
}

// validateManifest checks required fields on a parsed Manifest.
func validateManifest(m *Manifest) error {
	if m.Kind != "Employee" {
		return fmt.Errorf("unsupported kind %q: want \"Employee\"", m.Kind)
	}
	if m.Metadata.ID == "" {
		return fmt.Errorf("metadata.id is required")
	}
	if m.RoleID == "" {
		return fmt.Errorf("role_id is required")
	}
	return nil
}

// mustJSON marshals v to json.RawMessage; returns `{}` on a nil/empty map or
// on an unexpected marshal error (which should never happen for map[string]any).
func mustJSON(v map[string]any) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(b)
}

// firstNonEmpty returns the first non-empty string among the candidates.
func firstNonEmpty(candidates ...string) string {
	for _, s := range candidates {
		if s != "" {
			return s
		}
	}
	return ""
}
