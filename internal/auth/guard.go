package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nrynss/keel/wire"
	"github.com/nrynss/memento-emptor/internal/api/contract"
)

// The stable error codes a refusal carries. A client branches on the code
// and never on the message.
const (
	// CodeUnauthenticated is the code on every failed authentication: no
	// key, an unknown key, a revoked key, a bad scheme, and a key naming a
	// shopper it is not bound to. One code covers every case, so the answer
	// never says what was wrong or whether a shopper exists.
	CodeUnauthenticated = "unauthenticated"
	// CodeForbidden is the code for an authenticated caller asking for a
	// tool its kind may not use. It tells the holder of a key only that the
	// key it already holds is valid.
	CodeForbidden = "forbidden"
)

// The messages beside the codes. They are fixed for the same reason: the
// refusal must not vary with what was wrong.
const (
	messageUnauthenticated = "unauthorized"
	messageForbidden       = "forbidden"
)

// The headers the guard writes for the layer behind it. The layer is reached
// on loopback by the edge, so it trusts these and nothing else; the guard
// removes any a client sent before setting its own.
const (
	// HeaderCallerKind carries the caller's kind.
	HeaderCallerKind = "X-Memento-Caller-Kind"
	// HeaderCallerID carries the deployment's id for the key, never the
	// token.
	HeaderCallerID = "X-Memento-Caller-Id"
	// HeaderSeller carries the seller whose layer issued the key.
	HeaderSeller = "X-Memento-Seller"
	// HeaderShopper carries the shopper the request is scoped to, and is
	// absent unless the guard checked the caller against one.
	HeaderShopper = "X-Memento-Shopper"
)

// maxBodyBytes bounds what the guard reads from a request it must inspect.
// The layer refuses a larger body, so the guard stops there rather than
// buffer more.
const maxBodyBytes = 1 << 20

// Guard authenticates and authorizes every request in front of the layer. It
// is built once from a ring and is safe for concurrent use.
type Guard struct {
	ring *Keyring
	next http.Handler
}

// Guard wraps next with the keyring's checks. Every route of Surface is
// mounted through it at the edge, which is why an unresolvable method and
// path fails closed here too.
func (k *Keyring) Guard(next http.Handler) http.Handler {
	return &Guard{ring: k, next: next}
}

// ServeHTTP authenticates the caller, bounds the call to the shopper it may
// act for, and passes it on. The body it reads to find the tool and the
// shopper is put back untouched for the proxy.
func (g *Guard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stripIdentity(r)
	route, ok := Resolve(r.Method, r.URL.Path)
	if !ok {
		refuse(w)
		return
	}
	if route.Public {
		r.Header.Del("Authorization")
		g.next.ServeHTTP(w, r)
		return
	}
	principal, ok := g.ring.callersOf(r)
	if !ok {
		refuse(w)
		return
	}
	scope, err := readScope(route, r)
	if err != nil {
		writeInvalid(w)
		return
	}
	switch authorize(route, principal, scope) {
	case denyUnauthenticated:
		refuse(w)
		return
	case denyForbidden:
		forbid(w)
		return
	}
	applyPrincipal(r, principal, checkedShopper(route, scope))
	g.next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
}

// callersOf authenticates the request's bearer token. A request with no
// usable Authorization header, or one whose token names no live key, is not
// authenticated.
func (k *Keyring) callersOf(r *http.Request) (Principal, bool) {
	token, ok := bearerToken(r)
	if !ok {
		return Principal{}, false
	}
	return k.Authenticate(token)
}

// bearerToken reads one bearer token from the Authorization header. The
// scheme is matched case-insensitively, as HTTP allows; the token itself is
// taken exactly as sent.
func bearerToken(r *http.Request) (string, bool) {
	const scheme = "Bearer "
	value := r.Header.Get("Authorization")
	if len(value) <= len(scheme) || !strings.EqualFold(value[:len(scheme)], scheme) {
		return "", false
	}
	token := strings.TrimSpace(value[len(scheme):])
	if token == "" {
		return "", false
	}
	return token, true
}

// requestScope is what the guard reads out of a request's body to bound the
// call: the tool it names, when the route carries one, and the shopper it
// acts for.
type requestScope struct {
	// tool is the tool the request names, or the empty name.
	tool contract.ToolName
	// hasTool marks a route or method that named a tool.
	hasTool bool
	// shopper is the shopper id the request names, or empty. An empty
	// shopper is never one a caller may act for.
	shopper string
}

// readScope reads the fields the guard enforces and puts the body back for
// the proxy. A body that is not JSON names no tool and no shopper: the tools
// route then names its tool from the path and refuses the missing shopper,
// while the MCP route passes the body on and the layer's own decoder rejects
// it.
func readScope(route Route, r *http.Request) (requestScope, error) {
	if route.ID != RouteMCP && route.ID != RouteTools {
		return requestScope{}, nil
	}
	scope := requestScope{}
	if route.ID == RouteTools {
		scope.tool = contract.ToolName(strings.TrimPrefix(r.URL.Path, "/tools/"))
		scope.hasTool = scope.tool != ""
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		return requestScope{}, fmt.Errorf("auth: read body: %w", err)
	}
	if len(body) > maxBodyBytes {
		return requestScope{}, fmt.Errorf("auth: body is over %d bytes", maxBodyBytes)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 || !json.Valid(body) {
		return scope, nil
	}
	if route.ID == RouteMCP {
		readMCPBody(body, &scope)
	} else {
		scope.shopper = readShopperField(body)
	}
	return scope, nil
}

// readMCPBody reads a JSON-RPC request's tool name and, for tools/call, the
// shopper its arguments name. Any other method carries no scope.
func readMCPBody(body []byte, scope *requestScope) {
	var request struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(body, &request); err != nil || request.Method != "tools/call" {
		return
	}
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(request.Params, &call); err != nil || call.Name == "" {
		return
	}
	scope.tool = contract.ToolName(call.Name)
	scope.hasTool = true
	if len(call.Arguments) == 0 {
		return
	}
	scope.shopper = readShopperField(call.Arguments)
}

// readShopperField reads the shopper_id of a tool request. A body whose
// shopper_id is not a string, or is absent, reads as no shopper, which a
// scoped tool refuses.
func readShopperField(raw json.RawMessage) string {
	var request struct {
		ShopperID string `json:"shopper_id"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return ""
	}
	return request.ShopperID
}

// decision is one authorization answer.
type decision int

// The three answers the guard can reach.
const (
	// allow passes the call on.
	allow decision = iota
	// denyUnauthenticated answers with the fixed 401. It covers a caller
	// naming a shopper it is not bound to, so that answer is identical to
	// an unknown key's.
	denyUnauthenticated
	// denyForbidden answers with the 403. It covers a caller whose key is
	// valid but whose kind may not use this tool.
	denyForbidden
)

// authorize applies one route's rule to an authenticated caller.
func authorize(route Route, principal Principal, scope requestScope) decision {
	switch route.ID {
	case RouteCalls:
		if !allows(CallsAccess(), principal.Kind) {
			return denyForbidden
		}
		return allow
	case RouteMCP, RouteTools:
		if !scope.hasTool {
			// initialize, tools/list and notifications carry no shopper.
			// They read no memory, and any authenticated caller may ask.
			return allow
		}
		return authorizeTool(principal, scope)
	}
	return denyUnauthenticated
}

// authorizeTool applies one tool's rule. An unknown tool is allowed for any
// authenticated caller: the layer answers its own unknown-tool error, and a
// key cannot discover a tool by the guard's answer.
func authorizeTool(principal Principal, scope requestScope) decision {
	access, known := AccessFor(scope.tool)
	if !known {
		return allow
	}
	if !allows(access, principal.Kind) {
		return denyForbidden
	}
	if access.Scope == ScopeShopper && !principal.MayActFor(scope.shopper) {
		return denyUnauthenticated
	}
	return allow
}

// checkedShopper returns the shopper the guard actually checked the caller
// against, so the layer is told a shopper only when the call is bound to one.
// A tool whose rule names no shopper, and a tool the contract does not name,
// are never checked against the body's shopper_id: the header is then absent
// whatever the body said, so a caller cannot hand the layer a shopper it may
// not act for.
func checkedShopper(route Route, scope requestScope) string {
	if route.ID != RouteMCP && route.ID != RouteTools {
		return ""
	}
	if !scope.hasTool {
		return ""
	}
	access, known := AccessFor(scope.tool)
	if !known || access.Scope != ScopeShopper {
		return ""
	}
	return scope.shopper
}

// applyPrincipal sets the authenticated caller's identity headers. The
// token does not travel past the edge.
func applyPrincipal(r *http.Request, principal Principal, shopper string) {
	r.Header.Set(HeaderCallerKind, string(principal.Kind))
	r.Header.Set(HeaderCallerID, principal.KeyID)
	r.Header.Set(HeaderSeller, principal.Seller)
	if shopper != "" {
		r.Header.Set(HeaderShopper, shopper)
	}
	r.Header.Del("Authorization")
}

// stripIdentity removes every identity header a client sent before anything
// behind the edge sees the request. The guard sets its own headers only for
// an authenticated caller.
func stripIdentity(r *http.Request) {
	for _, name := range []string{HeaderCallerKind, HeaderCallerID, HeaderSeller, HeaderShopper} {
		r.Header.Del(name)
	}
}

// principalKey is the context key the authenticated caller travels under.
type principalKey struct{}

// withPrincipal attaches one authenticated caller to a context.
func withPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// PrincipalFrom returns the authenticated caller a guard attached to a
// request's context, and whether the request passed through one.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

// refuse answers every failed authentication with one fixed response. The
// status, the body and the headers never vary with what was wrong.
func refuse(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="memento-emptor"`)
	_ = wire.WriteError(w, http.StatusUnauthorized, CodeUnauthenticated, messageUnauthenticated, nil)
}

// forbid answers a valid caller asking for a tool its kind may not use.
func forbid(w http.ResponseWriter) {
	_ = wire.WriteError(w, http.StatusForbidden, CodeForbidden, messageForbidden, nil)
}

// writeInvalid answers a body the guard could not read at all.
func writeInvalid(w http.ResponseWriter) {
	_ = wire.WriteError(w, http.StatusBadRequest, wire.CodeInvalidRequest, "unreadable request body", nil)
}
