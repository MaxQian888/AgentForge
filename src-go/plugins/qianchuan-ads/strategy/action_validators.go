package strategy

import (
	"fmt"
	"strings"
)

// actionParamValidator is the per-action-type parameter shape check. It runs
// after the action type itself has been verified against ActionTypes.
type actionParamValidator func(actionType string, params map[string]any) error

var actionValidators = map[string]actionParamValidator{
	"adjust_bid":     validateBidLikeParams,
	"adjust_budget":  validateBidLikeParams,
	"pause_ad":       validateEmptyParams,
	"resume_ad":      validateEmptyParams,
	"apply_material": validateApplyMaterial,
	"notify_im":      validateNotifyIM,
	"record_event":   validateRecordEvent,
}

// ValidateAction enforces the per-type params contract. Errors always carry
// the action type prefix so the FE can surface them next to the offending
// rule.
func ValidateAction(a StrategyAction) error {
	v, ok := actionValidators[a.Type]
	if !ok {
		return fmt.Errorf("unknown action type %q (allowed: %s)", a.Type, strings.Join(ActionTypes, ", "))
	}
	return v(a.Type, a.Params)
}

// validateBidLikeParams enforces "exactly one of pct or to" used by
// adjust_bid and adjust_budget.
func validateBidLikeParams(actionType string, params map[string]any) error {
	pct, hasPct := params["pct"]
	to, hasTo := params["to"]
	if hasPct && hasTo {
		return fmt.Errorf("action %q: exactly one of pct or to required, got both", actionType)
	}
	if !hasPct && !hasTo {
		return fmt.Errorf("action %q: exactly one of pct or to required, got neither", actionType)
	}
	if hasPct {
		f, err := toFloat(pct)
		if err != nil {
			return fmt.Errorf("action %q: pct must be a number: %w", actionType, err)
		}
		if f == 0 {
			return fmt.Errorf("action %q: pct must be non-zero", actionType)
		}
		if f < -100 || f > 100 {
			return fmt.Errorf("action %q: pct must be in [-100, 100], got %v", actionType, f)
		}
	}
	if hasTo {
		f, err := toFloat(to)
		if err != nil {
			return fmt.Errorf("action %q: to must be a number: %w", actionType, err)
		}
		if f <= 0 {
			return fmt.Errorf("action %q: to must be positive, got %v", actionType, f)
		}
	}
	return nil
}

func validateEmptyParams(actionType string, params map[string]any) error {
	if len(params) > 0 {
		return fmt.Errorf("action %q must have empty params, got %d key(s)", actionType, len(params))
	}
	return nil
}

func validateApplyMaterial(actionType string, params map[string]any) error {
	if err := requireNonEmptyString(actionType, params, "material_id"); err != nil {
		return err
	}
	return nil
}

func validateNotifyIM(actionType string, params map[string]any) error {
	if err := requireNonEmptyString(actionType, params, "channel"); err != nil {
		return err
	}
	if err := requireNonEmptyString(actionType, params, "template"); err != nil {
		return err
	}
	return nil
}

func validateRecordEvent(actionType string, params map[string]any) error {
	return requireNonEmptyString(actionType, params, "event_name")
}

func requireNonEmptyString(actionType string, params map[string]any, key string) error {
	raw, ok := params[key]
	if !ok {
		return fmt.Errorf("action %q: missing required string param %q", actionType, key)
	}
	s, ok := raw.(string)
	if !ok {
		return fmt.Errorf("action %q: param %q must be a string, got %T", actionType, key, raw)
	}
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("action %q: param %q must be non-empty", actionType, key)
	}
	return nil
}

func toFloat(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}
