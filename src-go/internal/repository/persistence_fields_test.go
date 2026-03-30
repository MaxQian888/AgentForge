package repository

import (
	"encoding/json"
	"testing"
	"time"

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

func TestStringListAndScanHelpers(t *testing.T) {
	value := newStringList([]string{"alpha", "beta"})
	raw, err := value.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	var scanned stringList
	if err := scanned.Scan(raw); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(scanned.Slice()) != 2 || scanned.Slice()[0] != "alpha" {
		t.Fatalf("scanned.Slice() = %#v", scanned.Slice())
	}

	var nilJSON *jsonText
	if err := nilJSON.Scan("x"); err == nil {
		t.Fatal("jsonText nil receiver Scan() expected error")
	}

	var badRaw rawJSON
	if err := badRaw.Scan(123); err == nil {
		t.Fatal("rawJSON Scan(unsupported) expected error")
	}

	var nilList *stringList
	if err := nilList.Scan(raw); err == nil {
		t.Fatal("stringList nil receiver Scan() expected error")
	}

	if got := normalizeJSONRawMessage(nil, ""); got != nil {
		t.Fatalf("normalizeJSONRawMessage(nil, empty) = %#v, want nil", got)
	}

	original := json.RawMessage(`{"state":"ok"}`)
	cloned := cloneRawMessage(original)
	cloned[0] = '['
	if string(original) != `{"state":"ok"}` {
		t.Fatalf("cloneRawMessage mutated original: %s", string(original))
	}

	text := "hello"
	id := uuid.New()
	now := time.Now().UTC()
	if got := *cloneStringPointer(&text); got != text {
		t.Fatalf("cloneStringPointer() = %q, want %q", got, text)
	}
	if got := *cloneUUIDPointer(&id); got != id {
		t.Fatalf("cloneUUIDPointer() = %s, want %s", got, id)
	}
	if got := *cloneTimePointer(&now); !got.Equal(now) {
		t.Fatalf("cloneTimePointer() = %v, want %v", got, now)
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
