package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// QianchuanActionLog is the in-memory representation of one
// qianchuan_action_logs row. Each row tracks a single action emitted by the
// strategy runner and its lifecycle through the policy gate and executor.
type QianchuanActionLog struct {
	ID            uuid.UUID       `db:"id" json:"id"`
	BindingID     uuid.UUID       `db:"binding_id" json:"bindingId"`
	StrategyID    *uuid.UUID      `db:"strategy_id" json:"strategyId,omitempty"`
	StrategyRunID uuid.UUID       `db:"strategy_run_id" json:"strategyRunId"`
	RuleName      string          `db:"rule_name" json:"ruleName,omitempty"`
	ActionType    string          `db:"action_type" json:"actionType"`
	TargetAdID    string          `db:"target_ad_id" json:"targetAdId,omitempty"`
	Params        json.RawMessage `db:"params" json:"params"`
	Status        string          `db:"status" json:"status"`
	GateReason    string          `db:"gate_reason" json:"gateReason,omitempty"`
	AppliedAt     *time.Time      `db:"applied_at" json:"appliedAt,omitempty"`
	ErrorMessage  string          `db:"error_message" json:"errorMessage,omitempty"`
	CreatedAt     time.Time       `db:"created_at" json:"createdAt"`
}

// Action log status constants.
const (
	ActionLogStatusPending  = "pending"
	ActionLogStatusGated    = "gated"
	ActionLogStatusApplied  = "applied"
	ActionLogStatusRejected = "rejected"
	ActionLogStatusFailed   = "failed"
)
