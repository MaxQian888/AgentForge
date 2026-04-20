package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	rolepkg "github.com/agentforge/server/internal/role"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
)

type roleReferencePluginCatalogStub struct {
	records []*model.PluginRecord
}

func (s *roleReferencePluginCatalogStub) List(context.Context, service.PluginListFilter) ([]*model.PluginRecord, error) {
	return s.records, nil
}

type roleReferenceMemberCatalogStub struct {
	members []*model.Member
}

func (s *roleReferenceMemberCatalogStub) ListAll(context.Context) ([]*model.Member, error) {
	return s.members, nil
}

type roleReferenceQueueStoreStub struct {
	entries []*model.AgentPoolQueueEntry
}

func (s *roleReferenceQueueStoreStub) ListAllQueued(context.Context, int) ([]*model.AgentPoolQueueEntry, error) {
	return s.entries, nil
}

type roleReferenceRunCatalogStub struct {
	runs []*model.AgentRun
}

func (s *roleReferenceRunCatalogStub) ListByRole(context.Context, string, int) ([]*model.AgentRun, error) {
	return s.runs, nil
}

func TestRoleReferenceGovernanceService_InventoryGroupsBlockingAndAdvisoryConsumers(t *testing.T) {
	roleID := "frontend-developer"
	projectID := uuid.New()
	memberID := uuid.New()
	taskID := uuid.New()
	runID := uuid.New()
	now := time.Now().UTC()

	svc := service.NewRoleReferenceGovernanceService(
		&roleReferencePluginCatalogStub{
			records: []*model.PluginRecord{
				{
					PluginManifest: model.PluginManifest{
						APIVersion: "agentforge/v1",
						Kind:       model.PluginKindWorkflow,
						Metadata: model.PluginMetadata{
							ID:      "workflow.release-train",
							Name:    "Release Train",
							Version: "1.0.0",
						},
						Spec: model.PluginSpec{
							Workflow: &model.WorkflowPluginSpec{
								Process: model.WorkflowProcessSequential,
								Roles:   []model.WorkflowRoleBinding{{ID: roleID}},
								Steps:   []model.WorkflowStepDefinition{{ID: "implement", Role: roleID, Action: model.WorkflowActionAgent}},
							},
						},
					},
					LifecycleState: model.PluginStateEnabled,
				},
			},
		},
		&roleReferenceMemberCatalogStub{
			members: []*model.Member{
				{
					ID:          memberID,
					ProjectID:   projectID,
					Name:        "Frontend Bot",
					Type:        model.MemberTypeAgent,
					AgentConfig: `{"roleId":"frontend-developer","runtime":"codex"}`,
				},
			},
		},
		&roleReferenceQueueStoreStub{
			entries: []*model.AgentPoolQueueEntry{
				{
					EntryID:   "queue-1",
					ProjectID: projectID.String(),
					TaskID:    taskID.String(),
					MemberID:  memberID.String(),
					Status:    model.AgentPoolQueueStatusQueued,
					RoleID:    roleID,
					Runtime:   "codex",
					Provider:  "openai",
					Model:     "gpt-5-codex",
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
		},
		&roleReferenceRunCatalogStub{
			runs: []*model.AgentRun{
				{
					ID:        runID,
					TaskID:    taskID,
					MemberID:  memberID,
					RoleID:    roleID,
					Status:    model.AgentRunStatusCompleted,
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
		},
	)

	inventory, err := svc.ListReferences(context.Background(), roleID)
	if err != nil {
		t.Fatalf("ListReferences() error = %v", err)
	}

	if inventory.RoleID != roleID {
		t.Fatalf("inventory.RoleID = %q, want %q", inventory.RoleID, roleID)
	}
	if len(inventory.BlockingConsumers) != 3 {
		t.Fatalf("len(BlockingConsumers) = %d, want 3", len(inventory.BlockingConsumers))
	}
	if len(inventory.AdvisoryConsumers) != 1 {
		t.Fatalf("len(AdvisoryConsumers) = %d, want 1", len(inventory.AdvisoryConsumers))
	}
	if inventory.BlockingConsumers[0].ConsumerType == "" {
		t.Fatalf("blocking consumer missing type: %+v", inventory.BlockingConsumers[0])
	}
	if inventory.AdvisoryConsumers[0].ConsumerType != "historical-run" {
		t.Fatalf("advisory consumer = %+v, want historical-run", inventory.AdvisoryConsumers[0])
	}
}

func TestRoleReferenceGovernanceService_ValidateRoleBindingRejectsUnknownRole(t *testing.T) {
	roleStore := rolepkg.NewFileStore(t.TempDir())
	svc := service.NewRoleReferenceGovernanceService(nil, nil, nil, nil).WithRoleStore(roleStore)

	err := svc.ValidateRoleBinding(context.Background(), "missing-role")
	if err == nil {
		t.Fatal("ValidateRoleBinding() error = nil, want missing-role failure")
	}
}
