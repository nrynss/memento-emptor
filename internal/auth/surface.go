package auth

import (
	"strings"

	"github.com/nrynss/memento-emptor/internal/api/contract"
)

// RouteID names one route of the layer's public surface.
type RouteID string

// The routes the edge serves from the layer.
const (
	// RouteHealth is the deployment check. It is public and answers no
	// state.
	RouteHealth RouteID = "health"
	// RouteWebhook is the payment source's delivery path. It is public
	// because only a delivery the payment source verifies is accepted, and
	// verification happens behind the edge.
	RouteWebhook RouteID = "webhook"
	// RouteMCP is the MCP surface: JSON-RPC initialize, tools/list and
	// tools/call.
	RouteMCP RouteID = "mcp"
	// RouteTools serves one tool over plain HTTP for clients without an
	// MCP transport.
	RouteTools RouteID = "tools"
	// RouteCalls is the call log.
	RouteCalls RouteID = "calls"
)

// Route is one entry of the layer's public surface: the method and path the
// edge serves, and whether a caller must be authenticated.
type Route struct {
	// ID names the route.
	ID RouteID
	// Method is the HTTP method the route answers.
	Method string
	// Path is the route's path as a ServeMux pattern: a literal path, or
	// one with a single {tool} placeholder.
	Path string
	// Public marks a route that answers without a key. Only the health
	// check and the webhook are public.
	Public bool
}

// Pattern returns the route's ServeMux registration pattern.
func (r Route) Pattern() string {
	return r.Method + " " + r.Path
}

// Surface lists every route the edge serves. The guard is mounted on all of
// them; everything but the health check and the webhook refuses a request
// without a key.
func Surface() []Route {
	return []Route{
		{ID: RouteHealth, Method: "GET", Path: "/healthz", Public: true},
		{ID: RouteWebhook, Method: "POST", Path: "/webhooks/paypal", Public: true},
		{ID: RouteWebhook, Method: "POST", Path: "/webhooks/payment", Public: true},
		{ID: RouteMCP, Method: "POST", Path: "/mcp"},
		{ID: RouteTools, Method: "POST", Path: "/tools/{tool}"},
		{ID: RouteCalls, Method: "GET", Path: "/calls"},
	}
}

// Resolve maps a request's method and path to the surface route it names. A
// path the surface does not name is not resolved.
func Resolve(method, path string) (Route, bool) {
	for _, route := range Surface() {
		if route.Method != method {
			continue
		}
		if route.Path == path {
			return route, true
		}
		if suffix, ok := strings.CutSuffix(route.Path, "{tool}"); ok {
			rest, found := strings.CutPrefix(path, suffix)
			if found && rest != "" && !strings.Contains(rest, "/") {
				return route, true
			}
		}
	}
	return Route{}, false
}

// Scope says how a tool call is bounded to a shopper.
type Scope int

// The two scopes a tool can have.
const (
	// ScopeNone means the request names no shopper. The answer carries no
	// shopper's memory.
	ScopeNone Scope = iota
	// ScopeShopper means the request's shopper_id must be one the caller
	// may act for. A caller that names any other shopper is refused with
	// the same answer as an unknown key.
	ScopeShopper
)

// String names the scope, for messages and docs.
func (s Scope) String() string {
	switch s {
	case ScopeNone:
		return "none"
	case ScopeShopper:
		return "shopper"
	}
	return "unknown"
}

// ToolAccess is who may call one tool, and how the call is bounded.
type ToolAccess struct {
	// Callers are the caller kinds the tool admits.
	Callers []CallerKind
	// Scope is how the call is bounded to a shopper.
	Scope Scope
}

// AccessFor reports the access rule for one tool. A tool the contract does
// not name has no rule: any authenticated caller may ask, and the layer
// answers its own unknown-tool error.
func AccessFor(tool contract.ToolName) (ToolAccess, bool) {
	access, ok := toolAccess[tool]
	return access, ok
}

// toolAccess is the table: the tools a shopper-scoped caller reaches are
// the tools that act for one shopper, and product_fit is the pooled read,
// which carries no shopper's row.
//
// product_fit admits every kind because its answer is the pool: counts, a
// direction and a status, with no identity and no row. A shopper reads the
// same numbers recall already returns, and the merchant app reads them
// without a shopper. The merchant key is still good only for this tool,
// since it is bound to no shopper and the scoped tools refuse it.
var toolAccess = map[contract.ToolName]ToolAccess{
	contract.ToolRecallForPurchase: {Callers: []CallerKind{CallerAgent, CallerShopper}, Scope: ScopeShopper},
	contract.ToolRecordCorrection:  {Callers: []CallerKind{CallerAgent, CallerShopper}, Scope: ScopeShopper},
	contract.ToolPendingQuestions:  {Callers: []CallerKind{CallerAgent, CallerShopper}, Scope: ScopeShopper},
	contract.ToolRecordReason:      {Callers: []CallerKind{CallerAgent, CallerShopper}, Scope: ScopeShopper},
	contract.ToolRecordFeedback:    {Callers: []CallerKind{CallerAgent, CallerShopper}, Scope: ScopeShopper},
	contract.ToolExplainBelief:     {Callers: []CallerKind{CallerAgent, CallerShopper}, Scope: ScopeShopper},
	contract.ToolSetPooling:        {Callers: []CallerKind{CallerAgent, CallerShopper}, Scope: ScopeShopper},
	contract.ToolLedgerBeliefs:     {Callers: []CallerKind{CallerShopper}, Scope: ScopeShopper},
	contract.ToolLedgerBelief:      {Callers: []CallerKind{CallerShopper}, Scope: ScopeShopper},
	contract.ToolLedgerEvidence:    {Callers: []CallerKind{CallerShopper}, Scope: ScopeShopper},
	contract.ToolProductFit:        {Callers: []CallerKind{CallerAgent, CallerShopper, CallerMerchant}, Scope: ScopeNone},
}

// CallsAccess is the rule for the call log. The log carries every shopper's
// call entries and the served layer does not scope it per shopper, so only
// an agent key may read it: an agent is the seller's own service, while a
// shopper link token must never see another shopper's calls.
func CallsAccess() ToolAccess {
	return ToolAccess{Callers: []CallerKind{CallerAgent}, Scope: ScopeNone}
}

// allows reports whether one caller kind appears in an access rule.
func allows(access ToolAccess, kind CallerKind) bool {
	for _, allowed := range access.Callers {
		if allowed == kind {
			return true
		}
	}
	return false
}
