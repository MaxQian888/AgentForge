package strategy

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	maxNameLength    = 128
	scheduleMin      = 10 * time.Second
	scheduleMax      = time.Hour
	maxRuleNameChars = 128
)

// StrategyParseError is the structured error type returned by Parse for any
// validation or YAML-decoding failure. The FE editor consumes Line/Col to
// place markers and Field/Msg for the inline message.
type StrategyParseError struct {
	Line  int    `json:"line"`
	Col   int    `json:"col"`
	Field string `json:"field"`
	Msg   string `json:"msg"`
}

// Error implements the error interface.
func (e *StrategyParseError) Error() string {
	if e == nil {
		return ""
	}
	if e.Line > 0 {
		if e.Field != "" {
			return fmt.Sprintf("%s: %s (line %d, col %d)", e.Field, e.Msg, e.Line, e.Col)
		}
		return fmt.Sprintf("%s (line %d, col %d)", e.Msg, e.Line, e.Col)
	}
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Msg)
	}
	return e.Msg
}

// Parse parses a YAML strategy manifest, validates structure and content, and
// returns both the in-memory Strategy form and the runtime-optimized
// ParsedSpec. All errors are structured *StrategyParseError values so the
// HTTP layer can serialize them directly to the FE.
func Parse(yamlSource string) (*Strategy, *ParsedSpec, error) {
	// Pass 1: decode into a generic node tree to capture line/col on later errors.
	var rootNode yaml.Node
	if err := yaml.Unmarshal([]byte(yamlSource), &rootNode); err != nil {
		return nil, nil, yamlErrToStructured(err, "")
	}

	// Pass 2: typed decode into Strategy.
	var s Strategy
	if err := yaml.Unmarshal([]byte(yamlSource), &s); err != nil {
		return nil, nil, yamlErrToStructured(err, "")
	}

	if err := validateStrategy(&s, &rootNode); err != nil {
		return nil, nil, err
	}

	parsed, err := compileStrategy(&s)
	if err != nil {
		return nil, nil, err
	}
	return &s, parsed, nil
}

// nodePos resolves the line/col of a dotted YAML path against the parsed
// document tree. Path segments are either map keys or array indices encoded
// as their decimal form, e.g. "rules.0.actions.1.type". Missing paths return
// (0, 0).
func nodePos(root *yaml.Node, path string) (int, int) {
	if root == nil {
		return 0, 0
	}
	cur := root
	if cur.Kind == yaml.DocumentNode && len(cur.Content) > 0 {
		cur = cur.Content[0]
	}
	if path == "" {
		return cur.Line, cur.Column
	}
	for _, seg := range strings.Split(path, ".") {
		if cur == nil {
			return 0, 0
		}
		switch cur.Kind {
		case yaml.MappingNode:
			found := false
			for i := 0; i+1 < len(cur.Content); i += 2 {
				k := cur.Content[i]
				if k.Value == seg {
					cur = cur.Content[i+1]
					found = true
					break
				}
			}
			if !found {
				return 0, 0
			}
		case yaml.SequenceNode:
			idx := 0
			for _, r := range seg {
				if r < '0' || r > '9' {
					return 0, 0
				}
				idx = idx*10 + int(r-'0')
			}
			if idx < 0 || idx >= len(cur.Content) {
				return 0, 0
			}
			cur = cur.Content[idx]
		default:
			return 0, 0
		}
	}
	if cur == nil {
		return 0, 0
	}
	return cur.Line, cur.Column
}

// errAt builds a StrategyParseError annotated with positional info from the
// YAML document tree.
func errAt(root *yaml.Node, dotPath, fieldLabel, msg string) *StrategyParseError {
	line, col := nodePos(root, dotPath)
	return &StrategyParseError{Line: line, Col: col, Field: fieldLabel, Msg: msg}
}

func validateStrategy(s *Strategy, root *yaml.Node) error {
	if strings.TrimSpace(s.Name) == "" {
		return errAt(root, "name", "name", "name is required")
	}
	if len(s.Name) > maxNameLength {
		return errAt(root, "name", "name", fmt.Sprintf("name must be at most %d characters", maxNameLength))
	}

	if strings.TrimSpace(s.Triggers.Schedule) == "" {
		return errAt(root, "triggers.schedule", "triggers.schedule", "schedule is required")
	}
	d, err := time.ParseDuration(s.Triggers.Schedule)
	if err != nil {
		return errAt(root, "triggers.schedule", "triggers.schedule", fmt.Sprintf("invalid schedule duration %q: %v", s.Triggers.Schedule, err))
	}
	if d < scheduleMin || d > scheduleMax {
		return errAt(root, "triggers.schedule", "triggers.schedule",
			fmt.Sprintf("schedule %s out of range; must be in [%s, %s]", d, scheduleMin, scheduleMax))
	}

	for i, in := range s.Inputs {
		idxStr := fmt.Sprintf("%d", i)
		if strings.TrimSpace(in.Metric) == "" {
			return errAt(root, "inputs."+idxStr+".metric", fmt.Sprintf("inputs[%d].metric", i), "metric is required")
		}
		if strings.TrimSpace(in.Window) == "" {
			return errAt(root, "inputs."+idxStr+".window", fmt.Sprintf("inputs[%d].window", i), "window is required")
		}
		if _, err := time.ParseDuration(in.Window); err != nil {
			return errAt(root, "inputs."+idxStr+".window", fmt.Sprintf("inputs[%d].window", i),
				fmt.Sprintf("invalid duration %q: %v", in.Window, err))
		}
	}

	if len(s.Rules) == 0 {
		return errAt(root, "rules", "rules", "at least one rule required")
	}
	seen := make(map[string]int, len(s.Rules))
	for i, r := range s.Rules {
		idxStr := fmt.Sprintf("%d", i)
		if strings.TrimSpace(r.Name) == "" {
			return errAt(root, "rules."+idxStr+".name", fmt.Sprintf("rules[%d].name", i), "name is required")
		}
		if len(r.Name) > maxRuleNameChars {
			return errAt(root, "rules."+idxStr+".name", fmt.Sprintf("rules[%d].name", i),
				fmt.Sprintf("name must be at most %d characters", maxRuleNameChars))
		}
		if prev, ok := seen[r.Name]; ok {
			return errAt(root, "rules."+idxStr+".name", fmt.Sprintf("rules[%d].name", i),
				fmt.Sprintf("duplicate rule name %q (also at rules[%d])", r.Name, prev))
		}
		seen[r.Name] = i
		if strings.TrimSpace(r.Condition) == "" {
			return errAt(root, "rules."+idxStr+".condition", fmt.Sprintf("rules[%d].condition", i),
				fmt.Sprintf("rule %q: condition is required and must be non-empty", r.Name))
		}
		if len(r.Actions) == 0 {
			return errAt(root, "rules."+idxStr+".actions", fmt.Sprintf("rules[%d].actions", i),
				fmt.Sprintf("rule %q: at least one action required", r.Name))
		}
		for j, a := range r.Actions {
			jdxStr := fmt.Sprintf("%d", j)
			actionPath := "rules." + idxStr + ".actions." + jdxStr
			if !IsValidActionType(a.Type) {
				return errAt(root, actionPath+".type",
					fmt.Sprintf("rules[%d].actions[%d].type", i, j),
					fmt.Sprintf("rule %q: unknown action type %q (allowed: %s)", r.Name, a.Type, strings.Join(ActionTypes, ", ")))
			}
			if err := ValidateAction(a); err != nil {
				return errAt(root, actionPath,
					fmt.Sprintf("rules[%d].actions[%d]", i, j),
					fmt.Sprintf("rule %q: %v", r.Name, err))
			}
		}
	}
	return nil
}

func compileStrategy(s *Strategy) (*ParsedSpec, error) {
	d, _ := time.ParseDuration(s.Triggers.Schedule) // already validated
	parsed := &ParsedSpec{
		SchemaVersion:   ParsedSpecSchemaVersion,
		ScheduleSeconds: int(d / time.Second),
	}
	for _, in := range s.Inputs {
		w, _ := time.ParseDuration(in.Window)
		parsed.Inputs = append(parsed.Inputs, ParsedInput{
			Metric:        in.Metric,
			Dimensions:    append([]string(nil), in.Dimensions...),
			WindowSeconds: int(w / time.Second),
		})
	}
	for _, r := range s.Rules {
		pr := ParsedRule{Name: r.Name, ConditionRaw: r.Condition}
		for _, a := range r.Actions {
			pr.Actions = append(pr.Actions, ParsedAction{
				Type:     a.Type,
				AdIDExpr: a.Target.AdIDExpr,
				Params:   cloneParams(a.Params),
			})
		}
		parsed.Rules = append(parsed.Rules, pr)
	}
	return parsed, nil
}

func cloneParams(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// yamlErrToStructured promotes yaml.v3 typed errors into our StrategyParseError
// shape. yaml.TypeError carries one or more "line N: ..." messages; we extract
// the first line number we can find. Plain syntax errors land here too.
func yamlErrToStructured(err error, fieldHint string) error {
	if err == nil {
		return nil
	}
	var typeErr *yaml.TypeError
	msg := err.Error()
	line := extractLineNumber(msg)
	if errors.As(err, &typeErr) {
		if len(typeErr.Errors) > 0 {
			msg = typeErr.Errors[0]
			line = extractLineNumber(msg)
		}
	}
	return &StrategyParseError{
		Line:  line,
		Field: fieldHint,
		Msg:   msg,
	}
}

// extractLineNumber pulls "line N" out of a yaml.v3 error string. yaml.v3
// formats messages like "yaml: line 3: ..." or "line 3: ...". When nothing
// matches we return 0 and let the FE marker fall back to line 1.
func extractLineNumber(s string) int {
	idx := strings.Index(s, "line ")
	if idx < 0 {
		return 0
	}
	rest := s[idx+5:]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return 0
	}
	n := 0
	for _, r := range rest[:colon] {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
