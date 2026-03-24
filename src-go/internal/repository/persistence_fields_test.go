package repository

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestJSONStringRoundTrip(t *testing.T) {
	value := newJSONText("", "{}")
	raw, err := value.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if raw != "{}" {
		t.Fatalf("Value() = %#v, want {}", raw)
	}

	var scanned jsonText
	if err := scanned.Scan([]byte(`{"enabled":true}`)); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if scanned.String("{}") != `{"enabled":true}` {
		t.Fatalf("String() = %q", scanned.String("{}"))
	}
}

func TestRawJSONRoundTrip(t *testing.T) {
	value := newRawJSON(json.RawMessage(`{"state":"ok"}`), "")
	raw, err := value.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if raw != `{"state":"ok"}` {
		t.Fatalf("Value() = %#v", raw)
	}

	var scanned rawJSON
	if err := scanned.Scan([]byte(`["a","b"]`)); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if string(scanned.Bytes("[]")) != `["a","b"]` {
		t.Fatalf("Bytes() = %s", scanned.Bytes("[]"))
	}
}

func TestUUIDListRoundTrip(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	value := newUUIDList(ids)
	raw, err := value.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	var scanned uuidList
	if err := scanned.Scan(raw); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(scanned.Slice()) != len(ids) {
		t.Fatalf("len(scanned) = %d, want %d", len(scanned.Slice()), len(ids))
	}
}

func TestApplyPaginationHandlesNil(t *testing.T) {
	if got := applyPagination(nil, 10, 5); got != nil {
		t.Fatalf("applyPagination(nil) = %#v, want nil", got)
	}

	db := &gorm.DB{}
	if got := applyPagination(db, 0, 0); got != db {
		t.Fatalf("applyPagination should return original db when no paging provided")
	}
}
