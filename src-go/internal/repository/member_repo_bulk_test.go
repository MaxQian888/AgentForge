package repository

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestMemberRepositoryBulkUpdateStatusScopesToProjectAndReportsMissing(t *testing.T) {
	projectID := uuid.New()
	otherProjectID := uuid.New()
	targetMemberID := uuid.New()
	otherProjectMemberID := uuid.New()
	now := time.Now().UTC()

	db := openFoundationRepoTestDB(t, &memberRecord{})
	repo := NewMemberRepository(db)

	records := []*memberRecord{
		{
			ID:        targetMemberID,
			ProjectID: projectID,
			Name:      "Review Bot",
			Type:      model.MemberTypeAgent,
			Role:      "code-reviewer",
			Status:    model.MemberStatusActive,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        otherProjectMemberID,
			ProjectID: otherProjectID,
			Name:      "Other Bot",
			Type:      model.MemberTypeAgent,
			Role:      "observer",
			Status:    model.MemberStatusActive,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed member: %v", err)
		}
	}

	method := reflect.ValueOf(repo).MethodByName("BulkUpdateStatus")
	if !method.IsValid() {
		t.Fatalf("BulkUpdateStatus method is missing")
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(context.Background()),
		reflect.ValueOf(projectID),
		reflect.ValueOf([]uuid.UUID{targetMemberID, otherProjectMemberID, uuid.New()}),
		reflect.ValueOf(model.MemberStatusSuspended),
	})
	if len(results) != 2 {
		t.Fatalf("BulkUpdateStatus() returned %d values, want 2", len(results))
	}
	if errValue := results[1]; !errValue.IsNil() {
		t.Fatalf("BulkUpdateStatus() error = %v", errValue.Interface())
	}

	outcomes := results[0]
	if outcomes.Len() != 3 {
		t.Fatalf("len(outcomes) = %d, want 3", outcomes.Len())
	}

	first := outcomes.Index(0)
	if got := first.FieldByName("MemberID").String(); got != targetMemberID.String() {
		t.Fatalf("first.MemberID = %s, want %s", got, targetMemberID.String())
	}
	if !first.FieldByName("Success").Bool() {
		t.Fatalf("first.Success = false, want true")
	}
	if got := first.FieldByName("Status").String(); got != model.MemberStatusSuspended {
		t.Fatalf("first.Status = %s, want suspended", got)
	}

	second := outcomes.Index(1)
	if second.FieldByName("Success").Bool() {
		t.Fatalf("second.Success = true, want false for out-of-scope member")
	}
	if got := second.FieldByName("MemberID").String(); got != otherProjectMemberID.String() {
		t.Fatalf("second.MemberID = %s, want %s", got, otherProjectMemberID.String())
	}

	var updated memberRecord
	if err := db.First(&updated, "id = ?", targetMemberID).Error; err != nil {
		t.Fatalf("fetch updated member: %v", err)
	}
	if updated.Status != model.MemberStatusSuspended || updated.IsActive {
		t.Fatalf("updated member = %#v, want suspended inactive member", updated)
	}

	var untouched memberRecord
	if err := db.First(&untouched, "id = ?", otherProjectMemberID).Error; err != nil {
		t.Fatalf("fetch other project member: %v", err)
	}
	if untouched.Status != model.MemberStatusActive || !untouched.IsActive {
		t.Fatalf("other project member = %#v, want unchanged active member", untouched)
	}
}
