package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

// Starter is the minimum dependency Router needs from the workflow engine.
// In production satisfied by *service.DAGWorkflowService (Task 10 made its
// StartExecution take a StartOptions arg).
type Starter interface {
	StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID, opts service.StartOptions) (*model.WorkflowExecution, error)
}

// ListRepository is the read side of workflow_triggers that Router needs.
type ListRepository interface {
	ListEnabledBySource(ctx context.Context, source model.TriggerSource) ([]*model.WorkflowTrigger, error)
}

// Event is the source-agnostic payload the Router consumes.
type Event struct {
	Source model.TriggerSource
	// Data is the event payload (IM command args, cron $now, webhook body).
	// Template references like `{{$event.pr_url}}` resolve into this map.
	Data map[string]any
}

// imTriggerConfig is the typed shape of a trigger's Config JSON for IM source.
type imTriggerConfig struct {
	Platform      string   `json:"platform"`
	Command       string   `json:"command"`
	MatchRegex    string   `json:"match_regex"`
	ChatAllowlist []string `json:"chat_allowlist"`
}

// Router dispatches an incoming Event to every matching, enabled trigger
// and starts one workflow execution per match.
type Router struct {
	repo    ListRepository
	starter Starter
	idem    IdempotencyStore
}

// NewRouter returns a new Router backed by the provided dependencies.
func NewRouter(repo ListRepository, starter Starter, idem IdempotencyStore) *Router {
	return &Router{repo: repo, starter: starter, idem: idem}
}

// Route returns the number of executions started. Errors encountered while
// starting individual executions are logged and counted but do NOT abort the
// remaining dispatches — only the LAST error is returned so the caller sees
// something went wrong at the batch level.
func (r *Router) Route(ctx context.Context, ev Event) (int, error) {
	triggers, err := r.repo.ListEnabledBySource(ctx, ev.Source)
	if err != nil {
		return 0, err
	}

	var lastErr error
	started := 0

	for _, trigger := range triggers {
		// Step a: Match filter.
		if !matchesTrigger(trigger, ev) {
			continue
		}

		// Step b: Idempotency check.
		if trigger.IdempotencyKeyTemplate != "" && trigger.DedupeWindowSeconds > 0 {
			rendered := renderTemplate(trigger.IdempotencyKeyTemplate, ev.Data)
			var key string
			if rendered == nil {
				key = ""
			} else {
				key = fmt.Sprint(rendered)
			}
			seen, idemErr := r.idem.SeenWithin(ctx, key, time.Duration(trigger.DedupeWindowSeconds)*time.Second)
			if idemErr != nil {
				lastErr = idemErr
				continue
			}
			if seen {
				continue
			}
		}

		// Step c: Input mapping.
		seed, mappingErr := renderInputMapping(trigger.InputMapping, ev.Data)
		if mappingErr != nil {
			lastErr = mappingErr
			continue
		}

		// Step d: Start execution.
		triggerID := trigger.ID
		_, execErr := r.starter.StartExecution(ctx, trigger.WorkflowID, nil, service.StartOptions{
			Seed:        seed,
			TriggeredBy: &triggerID,
		})
		if execErr != nil {
			lastErr = execErr
			continue
		}

		started++
	}

	return started, lastErr
}

// matchesTrigger returns true if ev satisfies the trigger's filter conditions.
func matchesTrigger(trigger *model.WorkflowTrigger, ev Event) bool {
	switch trigger.Source {
	case model.TriggerSourceIM:
		return matchesIMTrigger(trigger, ev)
	case model.TriggerSourceSchedule:
		// Schedule triggers always match; the cron dispatcher pre-filters by trigger.
		return true
	default:
		return false
	}
}

// matchesIMTrigger checks the IM-specific filter conditions.
func matchesIMTrigger(trigger *model.WorkflowTrigger, ev Event) bool {
	if len(trigger.Config) == 0 {
		return true
	}

	var cfg imTriggerConfig
	if err := json.Unmarshal(trigger.Config, &cfg); err != nil {
		return false
	}

	// Platform filter.
	if cfg.Platform != "" {
		platform, _ := ev.Data["platform"].(string)
		if platform != cfg.Platform {
			return false
		}
	}

	// Command filter.
	if cfg.Command != "" {
		command, _ := ev.Data["command"].(string)
		if command != cfg.Command {
			return false
		}
	}

	// Regex filter on content.
	if cfg.MatchRegex != "" {
		content, _ := ev.Data["content"].(string)
		re, err := regexp.Compile(cfg.MatchRegex)
		if err != nil {
			return false
		}
		if !re.MatchString(content) {
			return false
		}
	}

	// Chat allowlist filter.
	if len(cfg.ChatAllowlist) > 0 {
		chatID, _ := ev.Data["chat_id"].(string)
		found := false
		for _, allowed := range cfg.ChatAllowlist {
			if chatID == allowed {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// templateExpr matches template expressions like `{{ $event.some.path }}`.
var templateExpr = regexp.MustCompile(`\{\{\s*\$event\.([a-zA-Z0-9_.]+)\s*\}\}`)

// renderTemplate renders a template string against event data.
// If the entire trimmed string is a single template expression, it returns
// the resolved value preserving its native type.
// Otherwise it performs string substitution, stringifying each resolved value.
// Unresolvable paths render as empty string in embedded mode or nil in whole-template mode.
// Malformed templates (unbalanced braces) are returned unchanged.
func renderTemplate(tmpl string, data map[string]any) any {
	trimmed := strings.TrimSpace(tmpl)
	if m := templateExpr.FindStringSubmatch(trimmed); m != nil && m[0] == trimmed {
		// Whole-template reference: preserve native type.
		return lookupPath(data, m[1])
	}
	// Embedded: stringify each match.
	return templateExpr.ReplaceAllStringFunc(tmpl, func(match string) string {
		m := templateExpr.FindStringSubmatch(match)
		if m == nil {
			return ""
		}
		v := lookupPath(data, m[1])
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	})
}

// lookupPath traverses root following dotted path segments.
// Numeric segments index into arrays. Returns nil if any segment is unresolvable.
func lookupPath(root map[string]any, path string) any {
	var cur any = root
	for _, seg := range strings.Split(path, ".") {
		switch v := cur.(type) {
		case map[string]any:
			cur = v[seg]
		case []any:
			var idx int
			if _, err := fmt.Sscanf(seg, "%d", &idx); err != nil || idx < 0 || idx >= len(v) {
				return nil
			}
			cur = v[idx]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

// renderInputMapping renders each string value in the mapping as a template,
// passing through non-string values unchanged.
func renderInputMapping(mappingRaw json.RawMessage, data map[string]any) (map[string]any, error) {
	if len(mappingRaw) == 0 {
		return map[string]any{}, nil
	}
	var mapping map[string]any
	if err := json.Unmarshal(mappingRaw, &mapping); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(mapping))
	for k, v := range mapping {
		if s, ok := v.(string); ok {
			out[k] = renderTemplate(s, data)
		} else {
			out[k] = v
		}
	}
	return out, nil
}
