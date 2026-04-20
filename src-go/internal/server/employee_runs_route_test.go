package server

import (
	"strings"
	"testing"
)

// TestEmployeeRunsRouteRegistered guards that the GET
// /api/v1/employees/:id/runs route is wired alongside the existing
// employee CRUD routes. Failing this test means a future refactor
// dropped the route — Spec 1A explicitly requires it for the FE
// dashboard.
func TestEmployeeRunsRouteRegistered(t *testing.T) {
	// routes.go is large; we sniff its source for the route mounting
	// line. This is a pragmatic guard, not a runtime test.
	src := mustReadFile(t, "routes.go")
	if !strings.Contains(src, `/employees/:id/runs`) {
		t.Fatalf("expected GET /employees/:id/runs to be registered in routes.go")
	}
	if !strings.Contains(src, "NewEmployeeRunsHandler") {
		t.Fatalf("expected NewEmployeeRunsHandler() to be invoked in routes.go")
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := readSourceFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
