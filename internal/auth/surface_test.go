package auth

import (
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
)

func TestSurfacePatternsAndResolve(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, route := range Surface() {
		if seen[route.Pattern()] {
			t.Fatalf("duplicate pattern %q", route.Pattern())
		}
		seen[route.Pattern()] = true
		if route.Method == "" || route.Path == "" {
			t.Fatalf("route %+v is incomplete", route)
		}
		if route.Public && route.ID != RouteHealth && route.ID != RouteWebhook {
			t.Fatalf("route %q is public but is not the health check or the webhook", route.Path)
		}
		if route.Public != (route.ID == RouteHealth || route.ID == RouteWebhook) {
			t.Fatalf("route %q has the wrong public mark", route.Path)
		}
		path := strings.Replace(route.Path, "{tool}", "recall_for_purchase", 1)
		resolved, ok := Resolve(route.Method, path)
		if !ok || resolved.Pattern() != route.Pattern() {
			t.Fatalf("Resolve(%q, %q) = %+v, %v; want %q", route.Method, path, resolved, ok, route.Path)
		}
	}

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "wrong method", method: "GET", path: "/mcp"},
		{name: "wrong method on the tools route", method: "GET", path: "/tools/recall_for_purchase"},
		{name: "the tools prefix alone", method: "POST", path: "/tools/"},
		{name: "an unknown path", method: "POST", path: "/ledger"},
		{name: "a nested path", method: "POST", path: "/tools/recall_for_purchase/extra"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if route, ok := Resolve(tc.method, tc.path); ok {
				t.Fatalf("Resolve(%q, %q) resolved to %+v", tc.method, tc.path, route)
			}
		})
	}
}

// TestEveryToolIsClassified proves the access table covers the whole frozen
// surface: a tool added to the contract fails this test until the guard
// classifies it, rather than silently admitting any authenticated caller.
func TestEveryToolIsClassified(t *testing.T) {
	t.Parallel()

	for _, tool := range contract.ToolNames {
		access, ok := AccessFor(tool)
		if !ok {
			t.Fatalf("tool %q has no access rule", tool)
		}
		if len(access.Callers) == 0 {
			t.Fatalf("tool %q admits no caller kind", tool)
		}
		for _, kind := range access.Callers {
			if !validKind(kind) {
				t.Fatalf("tool %q admits unknown kind %q", tool, kind)
			}
		}
	}
}

// TestTheCallLogIsAgentOnly proves the call log, which carries every
// shopper's entries, never answers a shopper token or a merchant key.
func TestTheCallLogIsAgentOnly(t *testing.T) {
	t.Parallel()

	access := CallsAccess()
	if len(access.Callers) != 1 || access.Callers[0] != CallerAgent {
		t.Fatalf("CallsAccess() admits %v, want the agent alone", access.Callers)
	}
	if access.Scope != ScopeNone {
		t.Fatalf("CallsAccess() scope = %v, want ScopeNone", access.Scope)
	}
}

// TestPooledReadAdmitsNoScope proves product_fit names no shopper, so the
// merchant key needs no binding.
func TestPooledReadAdmitsNoScope(t *testing.T) {
	t.Parallel()

	access, ok := AccessFor(contract.ToolProductFit)
	if !ok {
		t.Fatal("product_fit has no access rule")
	}
	if access.Scope != ScopeNone {
		t.Fatalf("product_fit scope = %v, want ScopeNone", access.Scope)
	}
	if !allows(access, CallerMerchant) {
		t.Fatal("product_fit refuses the merchant key")
	}
}
