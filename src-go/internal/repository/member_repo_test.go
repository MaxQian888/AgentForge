package repository

import (
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestNewMemberRecordNormalizesBlankAgentConfig(t *testing.T) {
	record := newMemberRecord(&model.Member{
		ID:          uuid.New(),
		ProjectID:   uuid.New(),
		Name:        "QA Bot",
		Type:        model.MemberTypeHuman,
		Role:        "Verifier",
		Email:       "qa-bot@example.com",
		AgentConfig: "",
		Skills:      []string{},
		IsActive:    true,
	})

	if got := record.AgentConfig.String("{}"); got != "{}" {
		t.Fatalf("AgentConfig = %q, want {}", got)
	}
}

func TestMemberRecordToModelPreservesSkillsAndAgentConfig(t *testing.T) {
	record := &memberRecord{
		ID:          uuid.New(),
		ProjectID:   uuid.New(),
		Name:        "Review Bot",
		Type:        model.MemberTypeAgent,
		Role:        "code-reviewer",
		Email:       "",
		AgentConfig: newJSONText(`{"roleId":"frontend-developer","runtime":"codex"}`, "{}"),
		Skills:      newStringList([]string{"review", "typescript"}),
		IsActive:    true,
	}

	member := record.toModel()
	if member.AgentConfig != `{"roleId":"frontend-developer","runtime":"codex"}` {
		t.Fatalf("AgentConfig = %q", member.AgentConfig)
	}
	if len(member.Skills) != 2 {
		t.Fatalf("len(Skills) = %d, want 2", len(member.Skills))
	}
}
