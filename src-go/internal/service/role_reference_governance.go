package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
)

var ErrRoleBindingNotFound = errors.New("role binding not found")

type RoleBindingValidationError struct {
	RoleID string
	Field  string
}

func (e *RoleBindingValidationError) Error() string {
	if e == nil {
		return "role binding not found"
	}
	return fmt.Sprintf("role binding %s no longer resolves from the authoritative role registry", e.RoleID)
}

func (e *RoleBindingValidationError) Unwrap() error {
	return ErrRoleBindingNotFound
}

type roleReferenceRoleStore interface {
	Get(id string) (*rolepkg.Manifest, error)
}

type roleReferencePluginCatalog interface {
	List(ctx context.Context, filter PluginListFilter) ([]*model.PluginRecord, error)
}

type roleReferenceMemberCatalog interface {
	ListAll(ctx context.Context) ([]*model.Member, error)
}

type roleReferenceQueueCatalog interface {
	ListAllQueued(ctx context.Context, limit int) ([]*model.AgentPoolQueueEntry, error)
}

type roleReferenceRunCatalog interface {
	ListByRole(ctx context.Context, roleID string, limit int) ([]*model.AgentRun, error)
}

type RoleReferenceGovernanceService struct {
	pluginCatalog roleReferencePluginCatalog
	memberCatalog roleReferenceMemberCatalog
	queueCatalog  roleReferenceQueueCatalog
	runCatalog    roleReferenceRunCatalog
	roleStore     roleReferenceRoleStore
}

func NewRoleReferenceGovernanceService(
	pluginCatalog roleReferencePluginCatalog,
	memberCatalog roleReferenceMemberCatalog,
	queueCatalog roleReferenceQueueCatalog,
	runCatalog roleReferenceRunCatalog,
) *RoleReferenceGovernanceService {
	return &RoleReferenceGovernanceService{
		pluginCatalog: pluginCatalog,
		memberCatalog: memberCatalog,
		queueCatalog:  queueCatalog,
		runCatalog:    runCatalog,
	}
}

func (s *RoleReferenceGovernanceService) WithRoleStore(store roleReferenceRoleStore) *RoleReferenceGovernanceService {
	s.roleStore = store
	return s
}

func (s *RoleReferenceGovernanceService) ValidateRoleBinding(ctx context.Context, roleID string) error {
	normalizedRoleID := strings.TrimSpace(roleID)
	if normalizedRoleID == "" {
		return nil
	}
	if s == nil || s.roleStore == nil {
		return &RoleBindingValidationError{RoleID: normalizedRoleID, Field: "roleId"}
	}
	_, err := s.roleStore.Get(normalizedRoleID)
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return &RoleBindingValidationError{RoleID: normalizedRoleID, Field: "roleId"}
	}
	return fmt.Errorf("load role binding %s: %w", normalizedRoleID, err)
}

func (s *RoleReferenceGovernanceService) ListReferences(ctx context.Context, roleID string) (*model.RoleReferenceInventory, error) {
	normalizedRoleID := strings.TrimSpace(roleID)
	inventory := &model.RoleReferenceInventory{
		RoleID: normalizedRoleID,
	}
	if normalizedRoleID == "" {
		return inventory, nil
	}

	if s != nil && s.pluginCatalog != nil {
		plugins, err := ListDependencyPlugins(ctx, s.pluginCatalog)
		if err != nil {
			return nil, err
		}
		for _, consumer := range BuildRolePluginConsumers(normalizedRoleID, plugins) {
			inventory.BlockingConsumers = append(inventory.BlockingConsumers, model.RoleReferenceConsumer{
				ConsumerType:   "plugin-binding",
				ConsumerID:     consumer.PluginID,
				Label:          strings.TrimSpace(consumer.PluginName),
				LifecycleState: consumer.Status,
				Blocking:       true,
				Reason:         consumer.Message,
				Remediation:    "Update or remove the plugin workflow binding before deleting this role.",
				References:     append([]string(nil), consumer.References...),
			})
		}
	}

	if s != nil && s.memberCatalog != nil {
		members, err := s.memberCatalog.ListAll(ctx)
		if err != nil {
			return nil, err
		}
		for _, member := range members {
			if member == nil || member.Type != model.MemberTypeAgent {
				continue
			}
			if ResolveMemberBoundRoleID(member) != normalizedRoleID {
				continue
			}
			inventory.BlockingConsumers = append(inventory.BlockingConsumers, model.RoleReferenceConsumer{
				ConsumerType:   "member-binding",
				ConsumerID:     member.ID.String(),
				Label:          strings.TrimSpace(member.Name),
				ProjectID:      member.ProjectID.String(),
				MemberID:       member.ID.String(),
				LifecycleState: model.NormalizeMemberStatus(member.Status, member.IsActive),
				Blocking:       true,
				Reason:         fmt.Sprintf("Agent member %s is still bound to role %s", member.Name, normalizedRoleID),
				Remediation:    "Rebind or clear the member's role assignment before deleting this role.",
			})
		}
	}

	if s != nil && s.queueCatalog != nil {
		entries, err := s.queueCatalog.ListAllQueued(ctx, 500)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry == nil || strings.TrimSpace(entry.RoleID) != normalizedRoleID {
				continue
			}
			inventory.BlockingConsumers = append(inventory.BlockingConsumers, model.RoleReferenceConsumer{
				ConsumerType:   "queued-execution",
				ConsumerID:     entry.EntryID,
				Label:          entry.EntryID,
				ProjectID:      entry.ProjectID,
				MemberID:       entry.MemberID,
				TaskID:         entry.TaskID,
				LifecycleState: string(entry.Status),
				Blocking:       true,
				Reason:         "Queued execution still references this role.",
				Remediation:    "Cancel or drain the queued execution before deleting this role.",
			})
		}
	}

	if s != nil && s.runCatalog != nil {
		runs, err := s.runCatalog.ListByRole(ctx, normalizedRoleID, 100)
		if err != nil {
			return nil, err
		}
		for _, run := range runs {
			if run == nil {
				continue
			}
			inventory.AdvisoryConsumers = append(inventory.AdvisoryConsumers, model.RoleReferenceConsumer{
				ConsumerType:   "historical-run",
				ConsumerID:     run.ID.String(),
				Label:          run.ID.String(),
				TaskID:         run.TaskID.String(),
				MemberID:       run.MemberID.String(),
				LifecycleState: run.Status,
				Blocking:       false,
				Reason:         "Historical agent run retains this role for audit context.",
				Remediation:    "Historical run records keep their stored role_id after deletion.",
			})
		}
	}

	sortRoleReferenceConsumers(inventory.BlockingConsumers)
	sortRoleReferenceConsumers(inventory.AdvisoryConsumers)
	return inventory, nil
}

func ResolveMemberBoundRoleID(member *model.Member) string {
	if member == nil {
		return ""
	}
	return ExtractRoleIDFromAgentConfig(member.AgentConfig)
}

func ResolveEffectiveRoleID(explicitRoleID string, member *model.Member) string {
	if normalized := strings.TrimSpace(explicitRoleID); normalized != "" {
		return normalized
	}
	return ResolveMemberBoundRoleID(member)
}

func ExtractRoleIDFromAgentConfig(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return ""
	}
	roleID, _ := parsed["roleId"].(string)
	return strings.TrimSpace(roleID)
}

func sortRoleReferenceConsumers(consumers []model.RoleReferenceConsumer) {
	slices.SortFunc(consumers, func(a, b model.RoleReferenceConsumer) int {
		if compare := strings.Compare(a.ConsumerType, b.ConsumerType); compare != 0 {
			return compare
		}
		return strings.Compare(a.ConsumerID, b.ConsumerID)
	})
}
