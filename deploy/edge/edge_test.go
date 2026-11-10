package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nrynss/memento-emptor/internal/auth"
)

// Test tokens. They are invented here, hashed into the fixture ring, and
// never a live key.
const (
	testAgentToken    = "test-edge-agent-token-not-a-live-key"
	testMerchantToken = "test-edge-merchant-token-not-a-live-key"
)

// spyCall is one request the layer behind the edge received.
type spyCall struct {
	method string
	path   string
	header http.Header
	body   string
}

// spyBackend records the requests the proxy forwards and answers 200.
type spyBackend struct {
	mu    sync.Mutex
	calls []spyCall
}

// ServeHTTP records one forwarded request.
func (s *spyBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.calls = append(s.calls, spyCall{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: string(body)})
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

// count reports how many requests reached the layer.
func (s *spyBackend) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// last returns the most recent forwarded request.
func (s *spyBackend) last() (spyCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return spyCall{}, false
	}
	return s.calls[len(s.calls)-1], true
}

// testRing is the keyring every edge test runs against: one agent of the
// demo seller, bound to one shopper, and the merchant app.
func testRing(t *testing.T) *auth.Keyring {
	t.Helper()
	env := map[string]string{
		auth.EnvAgentKeys:    "agent-chat|" + auth.HashToken(testAgentToken) + "|seller-demo|shopper_01|-",
		auth.EnvMerchantKeys: "merchant-brandx|" + auth.HashToken(testMerchantToken) + "|seller-demo|-|-",
	}
	ring, err := auth.FromEnv(func(name string) string { return env[name] })
	if err != nil {
		t.Fatalf("auth.FromEnv: %v", err)
	}
	return ring
}

// newTestMux builds the edge over a spy layer and a static dir holding the
// app shell, so a client-side route can be told from an API answer.
func newTestMux(t *testing.T) (*http.ServeMux, *spyBackend) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"index.html", "200.html"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("app shell"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	spy := &spyBackend{}
	ring := testRing(t)
	return newMux(dir, spy, ring.Guard), spy
}

// surfacePath fills a route pattern for a request.
func surfacePath(route auth.Route) string {
	return strings.Replace(route.Path, "{tool}", "recall_for_purchase", 1)
}

// surfaceBody is a body the route would accept once authenticated.
func surfaceBody(route auth.Route) string {
	switch route.ID {
	case auth.RouteTools:
		return `{"shopper_id":"shopper_01","brand_id":"brand-x"}`
	case auth.RouteMCP:
		return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"recall_for_purchase","arguments":{"shopper_id":"shopper_01"}}}`
	case auth.RouteWebhook:
		return `{"shopper_id":"shopper_01","event_id":"WH-1"}`
	}
	return ""
}

// TestEveryRouteIsBehindTheGuard proves the middleware is mounted on every
// route of the layer's surface: only the health check and the webhook answer
// without a key, and no other route reaches the layer.
func TestEveryRouteIsBehindTheGuard(t *testing.T) {
	t.Parallel()

	mux, spy := newTestMux(t)
	for _, route := range auth.Surface() {
		request := httptest.NewRequest(route.Method, surfacePath(route), strings.NewReader(surfaceBody(route)))
		recorder := httptest.NewRecorder()
		before := spy.count()
		mux.ServeHTTP(recorder, request)

		switch {
		case route.ID == auth.RouteHealth:
			if recorder.Code != http.StatusOK || recorder.Body.String() != `{"ok":true}` {
				t.Fatalf("GET /healthz = %d %q, want the health answer", recorder.Code, recorder.Body.String())
			}
			if spy.count() != before {
				t.Fatal("the health check reached the layer")
			}
		case route.Public:
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s %s = %d, want 200 without a key", route.Method, route.Path, recorder.Code)
			}
			if spy.count() != before+1 {
				t.Fatalf("%s %s did not reach the layer", route.Method, route.Path)
			}
		default:
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s = %d without a key, want 401", route.Method, route.Path, recorder.Code)
			}
			if spy.count() != before {
				t.Fatalf("%s %s reached the layer without a key", route.Method, route.Path)
			}
			if !strings.Contains(recorder.Body.String(), auth.CodeUnauthenticated) {
				t.Fatalf("%s %s body = %q, want the unauthenticated envelope", route.Method, route.Path, recorder.Body.String())
			}
		}
	}
}

// TestAnAgentKeyReachesTheLayer proves the edge forwards the authenticated
// caller's identity and the untouched body, and never the token.
func TestAnAgentKeyReachesTheLayer(t *testing.T) {
	t.Parallel()

	mux, spy := newTestMux(t)
	body := `{"shopper_id":"shopper_01","brand_id":"brand-x"}`
	request := httptest.NewRequest("POST", "/tools/recall_for_purchase", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+testAgentToken)
	request.Header.Set(auth.HeaderCallerID, "forged")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", recorder.Code, recorder.Body.String())
	}
	call, ok := spy.last()
	if !ok {
		t.Fatal("the request did not reach the layer")
	}
	if call.path != "/tools/recall_for_purchase" {
		t.Fatalf("forwarded path = %q, want the route unchanged", call.path)
	}
	if call.body != body {
		t.Fatalf("forwarded body = %q, want %q", call.body, body)
	}
	if got := call.header.Get(auth.HeaderCallerKind); got != string(auth.CallerAgent) {
		t.Fatalf("caller-kind header = %q, want agent", got)
	}
	if got := call.header.Get(auth.HeaderCallerID); got != "agent-chat" {
		t.Fatalf("caller-id header = %q, want agent-chat", got)
	}
	if got := call.header.Get(auth.HeaderSeller); got != "seller-demo" {
		t.Fatalf("seller header = %q, want seller-demo", got)
	}
	if got := call.header.Get(auth.HeaderShopper); got != "shopper_01" {
		t.Fatalf("shopper header = %q, want shopper_01", got)
	}
	if call.header.Get("Authorization") != "" {
		t.Fatal("the token reached the layer")
	}
}

// TestTheLedgerShellIsStillServed proves the static app and its client-side
// routes stay public while the API paths do not fall through to them.
func TestTheLedgerShellIsStillServed(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)
	for _, path := range []string{"/", "/ledger", "/ledger/beliefs"} {
		request := httptest.NewRequest("GET", path, nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || recorder.Body.String() != "app shell" {
			t.Fatalf("GET %s = %d %q, want the app shell", path, recorder.Code, recorder.Body.String())
		}
	}

	for _, path := range []string{"/mcp", "/mcp/", "/tools", "/healthz/extra"} {
		request := httptest.NewRequest("GET", path, nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d %q, want 404 rather than the app shell", path, recorder.Code, recorder.Body.String())
		}
	}

	// A method the surface does not serve on an API path is a 404 too: the
	// request must never fall through to the shell and be answered there.
	for _, request := range []struct {
		method string
		path   string
	}{
		{method: "GET", path: "/tools/recall_for_purchase"},
		{method: "POST", path: "/calls"},
		{method: "POST", path: "/tools/recall_for_purchase/"},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, strings.NewReader(`{}`)))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s %s = %d %q, want 404 rather than the app shell", request.method, request.path, recorder.Code, recorder.Body.String())
		}
	}
}
