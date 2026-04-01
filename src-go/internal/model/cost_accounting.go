package model

const (
	CostAccountingModeAuthoritativeTotal = "authoritative_total"
	CostAccountingModeEstimatedAPI       = "estimated_api_pricing"
	CostAccountingModePlanIncluded       = "plan_included"
	CostAccountingModeUnpriced           = "unpriced"
)

const (
	CostAccountingCoverageFull    = "full"
	CostAccountingCoveragePartial = "partial"
	CostAccountingCoverageNone    = "none"
)

type CostAccountingComponent struct {
	Model               string  `json:"model"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CostUsd             float64 `json:"cost_usd"`
	Source              string  `json:"source"`
}

type CostAccountingSnapshot struct {
	TotalCostUsd        float64                   `json:"total_cost_usd"`
	InputTokens         int64                     `json:"input_tokens"`
	OutputTokens        int64                     `json:"output_tokens"`
	CacheReadTokens     int64                     `json:"cache_read_tokens"`
	CacheCreationTokens int64                     `json:"cache_creation_tokens"`
	Mode                string                    `json:"mode"`
	Coverage            string                    `json:"coverage"`
	Source              string                    `json:"source"`
	Components          []CostAccountingComponent `json:"components,omitempty"`
}

func (c *CostAccountingSnapshot) Clone() *CostAccountingSnapshot {
	if c == nil {
		return nil
	}

	cloned := *c
	if len(c.Components) > 0 {
		cloned.Components = append([]CostAccountingComponent(nil), c.Components...)
	}
	return &cloned
}
