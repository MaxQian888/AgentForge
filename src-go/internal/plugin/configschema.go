package plugin

import (
	"fmt"
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
)

// ValidateConfig checks a plugin's config payload against the
// manifest-declared PluginConfigSchema. Nil schema accepts any shape
// (backward-compat). The validator is intentionally a minimal JSON
// Schema subset — just enough to catch typos and shape mismatches
// before they reach the plugin runtime — not a full draft-07
// implementation.
func ValidateConfig(schema *model.PluginConfigSchema, config map[string]any) error {
	if schema == nil {
		return nil
	}
	return validateValue("config", schema, config)
}

func validateValue(path string, schema *model.PluginConfigSchema, value any) error {
	if schema == nil {
		return nil
	}

	if len(schema.Enum) > 0 {
		if !enumMatches(schema.Enum, value) {
			return fmt.Errorf("%s: value %v is not in enum %v", path, value, schema.Enum)
		}
	}

	switch strings.ToLower(schema.Type) {
	case "", "any":
		// no type constraint
	case "object":
		obj, ok := asObject(value)
		if !ok {
			return fmt.Errorf("%s: expected object, got %T", path, value)
		}
		for _, key := range schema.Required {
			if _, present := obj[key]; !present {
				return fmt.Errorf("%s: missing required field %q", path, key)
			}
		}
		if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
			for key := range obj {
				if _, declared := schema.Properties[key]; !declared {
					return fmt.Errorf("%s: unknown field %q (additionalProperties: false)", path, key)
				}
			}
		}
		for key, sub := range schema.Properties {
			if childValue, present := obj[key]; present {
				if err := validateValue(path+"."+key, sub, childValue); err != nil {
					return err
				}
			}
		}
	case "array":
		arr, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array, got %T", path, value)
		}
		if schema.Items != nil {
			for i, item := range arr {
				if err := validateValue(fmt.Sprintf("%s[%d]", path, i), schema.Items, item); err != nil {
					return err
				}
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: expected string, got %T", path, value)
		}
	case "integer":
		if !isIntegerLike(value) {
			return fmt.Errorf("%s: expected integer, got %T", path, value)
		}
	case "number":
		if !isNumberLike(value) {
			return fmt.Errorf("%s: expected number, got %T", path, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean, got %T", path, value)
		}
	default:
		return fmt.Errorf("%s: unsupported schema type %q", path, schema.Type)
	}
	return nil
}

func asObject(value any) (map[string]any, bool) {
	if m, ok := value.(map[string]any); ok {
		return m, true
	}
	if m, ok := value.(map[string]interface{}); ok {
		return m, true
	}
	return nil, false
}

func isIntegerLike(value any) bool {
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return v == float32(int64(v))
	case float64:
		return v == float64(int64(v))
	}
	return false
}

func isNumberLike(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	}
	return false
}

func enumMatches(allowed []any, value any) bool {
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	return false
}
