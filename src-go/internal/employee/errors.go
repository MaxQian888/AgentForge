package employee

import "errors"

var (
	ErrEmployeeArchived   = errors.New("employee is archived")
	ErrEmployeePaused     = errors.New("employee is paused")
	ErrRoleNotFound       = errors.New("role manifest not found for employee.role_id")
	ErrEmployeeNameExists = errors.New("employee name already exists in project")
)
