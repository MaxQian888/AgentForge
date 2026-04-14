package repository

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type jsonText string

func newJSONText(raw string, fallback string) jsonText {
	return jsonText(normalizeJSONString(raw, fallback))
}

func (j jsonText) String(defaultValue string) string {
	return normalizeJSONString(string(j), defaultValue)
}

func (j jsonText) Value() (driver.Value, error) {
	trimmed := strings.TrimSpace(string(j))
	if trimmed == "" {
		return nil, nil
	}
	return trimmed, nil
}

func (j *jsonText) Scan(src any) error {
	if j == nil {
		return fmt.Errorf("jsonText destination is nil")
	}

	switch value := src.(type) {
	case nil:
		*j = ""
	case string:
		*j = jsonText(value)
	case []byte:
		*j = jsonText(string(value))
	default:
		return fmt.Errorf("scan jsonText: unsupported type %T", src)
	}
	return nil
}

type rawJSON json.RawMessage

func newRawJSON(raw json.RawMessage, fallback string) rawJSON {
	return rawJSON(normalizeJSONRawMessage(raw, fallback))
}

func (j rawJSON) Bytes(defaultValue string) json.RawMessage {
	return normalizeJSONRawMessage(json.RawMessage(j), defaultValue)
}

func (j rawJSON) Value() (driver.Value, error) {
	trimmed := strings.TrimSpace(string(j))
	if trimmed == "" {
		return nil, nil
	}
	return trimmed, nil
}

func (j *rawJSON) Scan(src any) error {
	if j == nil {
		return fmt.Errorf("rawJSON destination is nil")
	}

	switch value := src.(type) {
	case nil:
		*j = nil
	case string:
		*j = rawJSON(value)
	case []byte:
		*j = rawJSON(append([]byte(nil), value...))
	default:
		return fmt.Errorf("scan rawJSON: unsupported type %T", src)
	}
	return nil
}

type stringList []string

func newStringList(values []string) stringList {
	return stringList(cloneStringSlice(values))
}

func (s stringList) Slice() []string {
	return cloneStringSlice([]string(s))
}

func (s stringList) Value() (driver.Value, error) {
	return pq.StringArray(s.Slice()).Value()
}

func (s *stringList) Scan(src any) error {
	if s == nil {
		return fmt.Errorf("stringList destination is nil")
	}

	var values pq.StringArray
	if err := values.Scan(src); err != nil {
		return err
	}
	*s = stringList(cloneStringSlice([]string(values)))
	return nil
}

type uuidList []uuid.UUID

func newUUIDList(values []uuid.UUID) uuidList {
	return uuidList(cloneUUIDSlice(values))
}

func (u uuidList) Slice() []uuid.UUID {
	return cloneUUIDSlice([]uuid.UUID(u))
}

func (u uuidList) Value() (driver.Value, error) {
	values := make(pq.StringArray, 0, len(u))
	for _, id := range u {
		values = append(values, id.String())
	}
	return values.Value()
}

func (u *uuidList) Scan(src any) error {
	if u == nil {
		return fmt.Errorf("uuidList destination is nil")
	}

	var values pq.StringArray
	if err := values.Scan(src); err != nil {
		return err
	}

	ids := make([]uuid.UUID, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		id, err := uuid.Parse(trimmed)
		if err != nil {
			return fmt.Errorf("scan uuidList: %w", err)
		}
		ids = append(ids, id)
	}

	*u = uuidList(ids)
	return nil
}

func normalizeJSONString(raw string, fallback string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func normalizeJSONRawMessage(raw json.RawMessage, fallback string) json.RawMessage {
	if len(strings.TrimSpace(string(raw))) == 0 {
		if fallback == "" {
			return nil
		}
		return json.RawMessage(fallback)
	}
	return cloneRawMessage(raw)
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}

func cloneUUIDSlice(values []uuid.UUID) []uuid.UUID {
	if values == nil {
		return nil
	}
	return append([]uuid.UUID(nil), values...)
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func applyPagination(db *gorm.DB, limit int, offset int) *gorm.DB {
	if db == nil {
		return nil
	}
	if limit > 0 {
		db = db.Limit(limit)
	}
	if offset > 0 {
		db = db.Offset(offset)
	}
	return db
}
