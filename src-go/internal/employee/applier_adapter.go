package employee

import (
	"context"

	"github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
)

// ApplierAdapter bridges *employee.Service into the nodetypes.EmployeeSpawner
// interface used by the workflow EffectApplier. Keeping the adapter in this
// package (not in nodetypes) avoids a dependency cycle: nodetypes defines the
// interface it needs, and employee provides a type that satisfies it.
type ApplierAdapter struct {
	Svc *Service
}

func (a ApplierAdapter) Invoke(ctx context.Context, in nodetypes.EmployeeInvokeInput) (*nodetypes.EmployeeInvokeResult, error) {
	budget := in.BudgetUsd
	override := &budget
	if budget <= 0 {
		override = nil // let Service.Invoke apply its own default/prefs
	}
	res, err := a.Svc.Invoke(ctx, InvokeInput{
		EmployeeID:     in.EmployeeID,
		TaskID:         in.TaskID,
		ExecutionID:    in.ExecutionID,
		NodeID:         in.NodeID,
		BudgetOverride: override,
	})
	if err != nil {
		return nil, err
	}
	return &nodetypes.EmployeeInvokeResult{AgentRunID: res.AgentRunID}, nil
}
