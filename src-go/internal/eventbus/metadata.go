package eventbus

const (
	MetaChannels       = "channels"
	MetaSpanID         = "span_id"
	MetaTraceID        = "trace_id"
	MetaCausationID    = "causation_id"
	MetaCorrelationID  = "correlation_id"
	MetaUserID         = "user_id"
	MetaProjectID      = "project_id"
	MetaCausationDepth = "causation_depth"
)

func ensureMeta(e *Event) {
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
}

func GetChannels(e *Event) []string {
	v, ok := e.Metadata[MetaChannels]
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case []string:
		out := make([]string, len(x))
		copy(out, x)
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func SetChannels(e *Event, channels []string) {
	ensureMeta(e)
	e.Metadata[MetaChannels] = append([]string(nil), channels...)
}

func GetString(e *Event, key string) string {
	if e.Metadata == nil {
		return ""
	}
	if s, ok := e.Metadata[key].(string); ok {
		return s
	}
	return ""
}

func SetString(e *Event, key, value string) {
	ensureMeta(e)
	e.Metadata[key] = value
}

func GetCausationDepth(e *Event) int {
	if e.Metadata == nil {
		return 0
	}
	switch n := e.Metadata[MetaCausationDepth].(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func IncrementCausationDepth(e *Event) {
	ensureMeta(e)
	e.Metadata[MetaCausationDepth] = GetCausationDepth(e) + 1
}
