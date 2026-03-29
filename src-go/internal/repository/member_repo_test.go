package repository

import (
	"reflect"
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

func TestMemberContractsExposeDocumentedStatusAndIMIdentityFields(t *testing.T) {
	t.Helper()

	memberType := reflect.TypeOf(model.Member{})
	for _, fieldName := range []string{"Status", "IMPlatform", "IMUserID"} {
		if _, ok := memberType.FieldByName(fieldName); !ok {
			t.Fatalf("model.Member missing field %s", fieldName)
		}
	}

	dtoType := reflect.TypeOf(model.MemberDTO{})
	for _, fieldName := range []string{"Status", "IMPlatform", "IMUserID"} {
		if _, ok := dtoType.FieldByName(fieldName); !ok {
			t.Fatalf("model.MemberDTO missing field %s", fieldName)
		}
	}

	createType := reflect.TypeOf(model.CreateMemberRequest{})
	for _, fieldName := range []string{"Status", "IMPlatform", "IMUserID"} {
		if _, ok := createType.FieldByName(fieldName); !ok {
			t.Fatalf("model.CreateMemberRequest missing field %s", fieldName)
		}
	}

	updateType := reflect.TypeOf(model.UpdateMemberRequest{})
	for _, fieldName := range []string{"Status", "IMPlatform", "IMUserID"} {
		if _, ok := updateType.FieldByName(fieldName); !ok {
			t.Fatalf("model.UpdateMemberRequest missing field %s", fieldName)
		}
	}
}

func TestMemberRecordMapsCanonicalStatusToCompatibilityFields(t *testing.T) {
	memberValue := reflect.New(reflect.TypeOf(model.Member{})).Elem()
	setField := func(name string, value any) {
		field := memberValue.FieldByName(name)
		if !field.IsValid() {
			t.Fatalf("model.Member missing field %s", name)
		}
		field.Set(reflect.ValueOf(value))
	}

	setField("ID", uuid.New())
	setField("ProjectID", uuid.New())
	setField("Name", "Suspended Review Bot")
	setField("Type", model.MemberTypeAgent)
	setField("Role", "code-reviewer")
	setField("Email", "")
	setField("AgentConfig", `{"runtime":"codex"}`)
	setField("Skills", []string{"review"})
	setField("Status", "suspended")
	setField("IMPlatform", "feishu")
	setField("IMUserID", "ou_bot_123")
	setField("IsActive", false)

	member := memberValue.Addr().Interface().(*model.Member)
	record := newMemberRecord(member)

	recordValue := reflect.ValueOf(record).Elem()
	for _, fieldName := range []string{"Status", "IMPlatform", "IMUserID"} {
		if !recordValue.FieldByName(fieldName).IsValid() {
			t.Fatalf("memberRecord missing field %s", fieldName)
		}
	}

	roundTrip := record.toModel()
	roundTripValue := reflect.ValueOf(roundTrip).Elem()

	if got := roundTripValue.FieldByName("Status"); !got.IsValid() || got.String() != "suspended" {
		t.Fatalf("roundTrip status = %#v, want suspended", got)
	}
	if got := roundTripValue.FieldByName("IMPlatform"); !got.IsValid() || got.String() != "feishu" {
		t.Fatalf("roundTrip im platform = %#v, want feishu", got)
	}
	if got := roundTripValue.FieldByName("IMUserID"); !got.IsValid() || got.String() != "ou_bot_123" {
		t.Fatalf("roundTrip im user id = %#v, want ou_bot_123", got)
	}
	if !roundTripValue.FieldByName("IsActive").IsValid() || roundTripValue.FieldByName("IsActive").Bool() {
		t.Fatalf("roundTrip isActive = %v, want false", roundTripValue.FieldByName("IsActive"))
	}
}
