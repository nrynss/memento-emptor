package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captured is what the handler behind the guard saw.
type captured struct {
	reached      bool
	header       http.Header
	body         []byte
	principal    Principal
	hasPrincipal bool
}

// captureInto returns a handler that records one request and answers 200.
func captureInto(got *captured) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "unreadable body", http.StatusInternalServerError)
			return
		}
		got.reached = true
		got.header = r.Header.Clone()
		got.body = body
		got.principal, got.hasPrincipal = PrincipalFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

// serve drives one request through the guard.
func serve(ring *Keyring, method, path, body, token string) (*httptest.ResponseRecorder, *captured) {
	got := &captured{}
	guard := ring.Guard(captureInto(got))
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	guard.ServeHTTP(recorder, request)
	return recorder, got
}

// recallBody names one shopper in a recall request.
func recallBody(shopper string) string {
	return `{"shopper_id":"` + shopper + `","brand_id":"brand-x"}`
}

// mcpCallBody wraps one tool call in JSON-RPC.
func mcpCallBody(tool, arguments string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + arguments + `}}`
}

// productFitBody asks for the pool of one line and size.
const productFitBody = `{"brand_id":"brand-x","category":"shirts","fit_handle":"regular","size":"M"}`

// productFitWithShopper is a pool read whose body also names a shopper. The
// tool's rule names none, so the guard must not pass the name on.
const productFitWithShopper = `{"shopper_id":"shopper_09","brand_id":"brand-x","category":"shirts","fit_handle":"regular","size":"M"}`

// errorCode reads one refusal's stable code.
func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("the refusal body is not the error envelope: %q", body)
	}
	return envelope.Error.Code
}

// TestGuardTable covers every caller kind against every answer the guard
// gives: a good key, a wrong key, a revoked key, and a good key asking for
// another seller's shopper.
func TestGuardTable(t *testing.T) {
	t.Parallel()

	tooLarge := strings.Repeat("a", maxBodyBytes+1)

	tests := []struct {
		name string
		// request
		method     string
		path       string
		body       string
		token      string
		authHeader string // used instead of a bearer token when set
		// want
		wantStatus  int
		wantCode    string
		wantReached bool
		wantKind    CallerKind
		wantKeyID   string
		wantSeller  string
		wantShopper string
	}{
		{
			name: "agent good key for a bound shopper", method: "POST", path: "/tools/recall_for_purchase",
			body: recallBody("shopper_01"), token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo", wantShopper: "shopper_01",
		},
		{
			name: "agent good key for its second bound shopper", method: "POST", path: "/tools/recall_for_purchase",
			body: recallBody("shopper_02"), token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo", wantShopper: "shopper_02",
		},
		{
			name: "agent with no key", method: "POST", path: "/tools/recall_for_purchase",
			body:       recallBody("shopper_01"),
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "agent with an unknown key", method: "POST", path: "/tools/recall_for_purchase",
			body: recallBody("shopper_01"), token: tokenUnknown,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "agent with a revoked key", method: "POST", path: "/tools/recall_for_purchase",
			body: recallBody("shopper_01"), token: tokenAgentRetired,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "agent good key for another seller's shopper", method: "POST", path: "/tools/recall_for_purchase",
			body: recallBody("shopper_09"), token: tokenAgentChat,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "agent good key for a shopper nobody serves", method: "POST", path: "/tools/recall_for_purchase",
			body: recallBody("shopper_nobody"), token: tokenAgentChat,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "agent good key with no shopper in the request", method: "POST", path: "/tools/recall_for_purchase",
			body: `{"brand_id":"brand-x"}`, token: tokenAgentChat,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "agent good key with a shopper that is not a string", method: "POST", path: "/tools/recall_for_purchase",
			body: `{"shopper_id":7,"brand_id":"brand-x"}`, token: tokenAgentChat,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "agent key sent with the wrong scheme", method: "POST", path: "/tools/recall_for_purchase",
			body: recallBody("shopper_01"), authHeader: "Basic " + tokenAgentChat,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "agent good key on the call log", method: "GET", path: "/calls", token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo",
		},
		{
			name: "agent good key on product fit", method: "POST", path: "/tools/product_fit",
			body: productFitBody, token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo",
		},
		{
			name: "agent good key on an unknown tool", method: "POST", path: "/tools/not_a_tool",
			body: `{}`, token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo",
		},
		{
			name: "agent good key on product fit naming another seller's shopper", method: "POST", path: "/tools/product_fit",
			body: productFitWithShopper, token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo",
		},
		{
			name: "agent good key on an unknown tool naming another seller's shopper", method: "POST", path: "/tools/not_a_tool",
			body: recallBody("shopper_09"), token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo",
		},
		{
			name: "shopper good token on their own ledger", method: "POST", path: "/tools/ledger_beliefs",
			body: `{"shopper_id":"shopper_01","include_seeded":true}`, token: tokenShopperDemo,
			wantStatus: 200, wantReached: true, wantKind: CallerShopper, wantKeyID: "shopper-demo",
			wantSeller: "seller-demo", wantShopper: "shopper_01",
		},
		{
			name: "shopper good token on another shopper's ledger", method: "POST", path: "/tools/ledger_beliefs",
			body: `{"shopper_id":"shopper_01","include_seeded":true}`, token: tokenShopperSecond,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "shopper with a revoked token", method: "POST", path: "/tools/ledger_beliefs",
			body: `{"shopper_id":"shopper_01","include_seeded":true}`, token: tokenShopperRetire,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "shopper with an unknown token", method: "POST", path: "/tools/ledger_beliefs",
			body: `{"shopper_id":"shopper_01","include_seeded":true}`, token: tokenUnknown,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "shopper good token on their own pooling choice", method: "POST", path: "/tools/set_pooling",
			body: `{"shopper_id":"shopper_01","pooling":false}`, token: tokenShopperDemo,
			wantStatus: 200, wantReached: true, wantKind: CallerShopper, wantKeyID: "shopper-demo",
			wantSeller: "seller-demo", wantShopper: "shopper_01",
		},
		{
			name: "shopper good token on another shopper's pooling choice", method: "POST", path: "/tools/set_pooling",
			body: `{"shopper_id":"shopper_01","pooling":false}`, token: tokenShopperSecond,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "shopper good token on their own correction", method: "POST", path: "/tools/record_correction",
			body: `{"shopper_id":"shopper_01","text":"I have lost weight, use M"}`, token: tokenShopperDemo,
			wantStatus: 200, wantReached: true, wantKind: CallerShopper, wantKeyID: "shopper-demo",
			wantSeller: "seller-demo", wantShopper: "shopper_01",
		},
		{
			name: "shopper good token on the call log", method: "GET", path: "/calls", token: tokenShopperDemo,
			wantStatus: 403, wantCode: CodeForbidden,
		},
		{
			name: "shopper good token on product fit", method: "POST", path: "/tools/product_fit",
			body: productFitBody, token: tokenShopperDemo,
			wantStatus: 200, wantReached: true, wantKind: CallerShopper, wantKeyID: "shopper-demo",
			wantSeller: "seller-demo",
		},
		{
			name: "merchant good key on product fit", method: "POST", path: "/tools/product_fit",
			body: productFitBody, token: tokenMerchant,
			wantStatus: 200, wantReached: true, wantKind: CallerMerchant, wantKeyID: "merchant-brandx",
			wantSeller: "seller-demo",
		},
		{
			name: "merchant good key on product fit naming a shopper", method: "POST", path: "/tools/product_fit",
			body: productFitWithShopper, token: tokenMerchant,
			wantStatus: 200, wantReached: true, wantKind: CallerMerchant, wantKeyID: "merchant-brandx",
			wantSeller: "seller-demo",
		},
		{
			name: "merchant good key on recall", method: "POST", path: "/tools/recall_for_purchase",
			body: recallBody("shopper_01"), token: tokenMerchant,
			wantStatus: 403, wantCode: CodeForbidden,
		},
		{
			name: "merchant good key on a ledger read", method: "POST", path: "/tools/ledger_beliefs",
			body: `{"shopper_id":"shopper_01","include_seeded":true}`, token: tokenMerchant,
			wantStatus: 403, wantCode: CodeForbidden,
		},
		{
			name: "merchant good key on the call log", method: "GET", path: "/calls", token: tokenMerchant,
			wantStatus: 403, wantCode: CodeForbidden,
		},
		{
			name: "merchant with a revoked key", method: "POST", path: "/tools/product_fit",
			body: productFitBody, token: tokenMerchantRetire,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "merchant with an unknown key", method: "POST", path: "/tools/product_fit",
			body: productFitBody, token: tokenUnknown,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "mcp tools call by an agent for a bound shopper", method: "POST", path: "/mcp",
			body: mcpCallBody("recall_for_purchase", `{"shopper_id":"shopper_01"}`), token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo", wantShopper: "shopper_01",
		},
		{
			name: "mcp tools call by an agent for another seller's shopper", method: "POST", path: "/mcp",
			body: mcpCallBody("recall_for_purchase", `{"shopper_id":"shopper_09"}`), token: tokenAgentChat,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "mcp tools call by a shopper for another shopper", method: "POST", path: "/mcp",
			body: mcpCallBody("ledger_beliefs", `{"shopper_id":"shopper_01"}`), token: tokenShopperSecond,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "mcp initialize with an agent key", method: "POST", path: "/mcp",
			body: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`, token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo",
		},
		{
			name: "mcp initialize with no key", method: "POST", path: "/mcp",
			body:       `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "mcp tools list with a shopper token", method: "POST", path: "/mcp",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, token: tokenShopperDemo,
			wantStatus: 200, wantReached: true, wantKind: CallerShopper, wantKeyID: "shopper-demo",
			wantSeller: "seller-demo",
		},
		{
			name: "mcp initialize with a merchant key", method: "POST", path: "/mcp",
			body: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`, token: tokenMerchant,
			wantStatus: 200, wantReached: true, wantKind: CallerMerchant, wantKeyID: "merchant-brandx",
			wantSeller: "seller-demo",
		},
		{
			name: "mcp unknown tool with an agent key", method: "POST", path: "/mcp",
			body: mcpCallBody("not_a_tool", `{}`), token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo",
		},
		{
			name: "mcp body that is not JSON with an agent key", method: "POST", path: "/mcp",
			body: "not json", token: tokenAgentChat,
			wantStatus: 200, wantReached: true, wantKind: CallerAgent, wantKeyID: "agent-chat",
			wantSeller: "seller-demo",
		},
		{
			name: "webhook with no key", method: "POST", path: "/webhooks/paypal",
			body:       `{"shopper_id":"shopper_01","event_id":"WH-1"}`,
			wantStatus: 200, wantReached: true,
		},
		{
			name: "webhook with a bogus key", method: "POST", path: "/webhooks/payment",
			body: `{"shopper_id":"shopper_01","event_id":"WH-1"}`, token: tokenUnknown,
			wantStatus: 200, wantReached: true,
		},
		{
			name: "health check with no key", method: "GET", path: "/healthz",
			wantStatus: 200, wantReached: true,
		},
		{
			name: "a path the surface does not name", method: "GET", path: "/mcp", token: tokenAgentChat,
			wantStatus: 401, wantCode: CodeUnauthenticated,
		},
		{
			name: "a body over the limit with a good key", method: "POST", path: "/tools/recall_for_purchase",
			body: tooLarge, token: tokenAgentChat,
			wantStatus: 400,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ring := testRing(t)
			got := &captured{}
			guard := ring.Guard(captureInto(got))
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.token != "" {
				request.Header.Set("Authorization", "Bearer "+tc.token)
			}
			if tc.authHeader != "" {
				request.Header.Set("Authorization", tc.authHeader)
			}
			recorder := httptest.NewRecorder()
			guard.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if tc.wantCode != "" {
				if code := errorCode(t, recorder.Body.Bytes()); code != tc.wantCode {
					t.Fatalf("error code = %q, want %q", code, tc.wantCode)
				}
			}
			if got.reached != tc.wantReached {
				t.Fatalf("handler reached = %v, want %v", got.reached, tc.wantReached)
			}
			if !tc.wantReached {
				return
			}
			if !got.hasPrincipal && tc.wantKeyID != "" {
				t.Fatal("the handler saw no principal")
			}
			if tc.wantKeyID != "" && got.principal.KeyID != tc.wantKeyID {
				t.Fatalf("principal key id = %q, want %q", got.principal.KeyID, tc.wantKeyID)
			}
			if tc.wantKind != "" && got.header.Get(HeaderCallerKind) != string(tc.wantKind) {
				t.Fatalf("caller-kind header = %q, want %q", got.header.Get(HeaderCallerKind), tc.wantKind)
			}
			if tc.wantSeller != "" && got.header.Get(HeaderSeller) != tc.wantSeller {
				t.Fatalf("seller header = %q, want %q", got.header.Get(HeaderSeller), tc.wantSeller)
			}
			if got.header.Get(HeaderShopper) != tc.wantShopper {
				t.Fatalf("shopper header = %q, want %q", got.header.Get(HeaderShopper), tc.wantShopper)
			}
			if got.header.Get("Authorization") != "" {
				t.Fatal("the token travelled past the edge")
			}
			if string(got.body) != tc.body {
				t.Fatalf("the body behind the guard = %q, want %q", got.body, tc.body)
			}
		})
	}
}

// TestRefusalsAreIdentical proves that an unknown key, a revoked key, a
// wrong scheme, a good key naming another seller's shopper and a good key
// naming nobody all answer with the same bytes.
func TestRefusalsAreIdentical(t *testing.T) {
	t.Parallel()

	ring := testRing(t)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		token  string
	}{
		{name: "no key", method: "POST", path: "/tools/recall_for_purchase", body: recallBody("shopper_01")},
		{name: "unknown key", method: "POST", path: "/tools/recall_for_purchase", body: recallBody("shopper_01"), token: tokenUnknown},
		{name: "revoked agent key", method: "POST", path: "/tools/recall_for_purchase", body: recallBody("shopper_01"), token: tokenAgentRetired},
		{name: "revoked shopper token", method: "POST", path: "/tools/ledger_beliefs", body: `{"shopper_id":"shopper_01"}`, token: tokenShopperRetire},
		{name: "revoked merchant key", method: "POST", path: "/tools/product_fit", body: productFitBody, token: tokenMerchantRetire},
		{name: "another seller's shopper", method: "POST", path: "/tools/recall_for_purchase", body: recallBody("shopper_09"), token: tokenAgentChat},
		{name: "a shopper the caller does not serve", method: "POST", path: "/tools/recall_for_purchase", body: recallBody("shopper_nobody"), token: tokenAgentChat},
		{name: "another shopper's ledger", method: "POST", path: "/tools/ledger_beliefs", body: `{"shopper_id":"shopper_01"}`, token: tokenShopperSecond},
		{name: "mcp call for another seller's shopper", method: "POST", path: "/mcp", body: mcpCallBody("recall_for_purchase", `{"shopper_id":"shopper_09"}`), token: tokenAgentChat},
		{name: "a path the surface does not name", method: "GET", path: "/mcp", token: tokenAgentChat},
	}
	var reference *httptest.ResponseRecorder
	for _, tc := range cases {
		recorder, got := serve(ring, tc.method, tc.path, tc.body, tc.token)
		if recorder.Code != 401 {
			t.Fatalf("%s: status = %d, want 401", tc.name, recorder.Code)
		}
		if got.reached {
			t.Fatalf("%s: the request reached the layer", tc.name)
		}
		if reference == nil {
			reference = recorder
			continue
		}
		if recorder.Body.String() != reference.Body.String() {
			t.Fatalf("%s: body = %q, want %q", tc.name, recorder.Body.String(), reference.Body.String())
		}
		if recorder.Header().Get("WWW-Authenticate") != reference.Header().Get("WWW-Authenticate") {
			t.Fatalf("%s: the refusal header differs", tc.name)
		}
		if recorder.Header().Get("Content-Type") != reference.Header().Get("Content-Type") {
			t.Fatalf("%s: the refusal content type differs", tc.name)
		}
		if !headersEqual(recorder.Header(), reference.Header()) {
			t.Fatalf("%s: headers = %v, want %v", tc.name, recorder.Header(), reference.Header())
		}
	}
}

// headersEqual reports whether two responses carry the same headers and the
// same values, so a header added on one refusal path and not another fails.
func headersEqual(a, b http.Header) bool {
	if len(a) != len(b) {
		return false
	}
	for name, values := range a {
		other, ok := b[name]
		if !ok || len(values) != len(other) {
			return false
		}
		for i := range values {
			if values[i] != other[i] {
				return false
			}
		}
	}
	return true
}

// TestForbiddenDoesNotVaryWithTheShopper proves the 403 for a valid key whose
// kind may not use a tool does not change with the shopper the body names, so
// it cannot be used to learn whether a shopper exists.
func TestForbiddenDoesNotVaryWithTheShopper(t *testing.T) {
	t.Parallel()

	ring := testRing(t)
	bodies := []string{
		recallBody("shopper_01"),
		recallBody("shopper_09"),
		recallBody("shopper_nobody"),
		`{"brand_id":"brand-x"}`,
	}
	var reference *httptest.ResponseRecorder
	for _, body := range bodies {
		recorder, got := serve(ring, "POST", "/tools/recall_for_purchase", body, tokenMerchant)
		if recorder.Code != 403 {
			t.Fatalf("body %q: status = %d, want 403", body, recorder.Code)
		}
		if got.reached {
			t.Fatalf("body %q: the merchant key reached a shopper's tool", body)
		}
		if reference == nil {
			reference = recorder
			continue
		}
		if recorder.Body.String() != reference.Body.String() || !headersEqual(recorder.Header(), reference.Header()) {
			t.Fatalf("body %q: the refusal differs from the first one", body)
		}
	}
}

// TestNoRouteAnswersWithoutAKey drives every route of the surface with no key
// and shows that only the health check and the webhook answer.
func TestNoRouteAnswersWithoutAKey(t *testing.T) {
	t.Parallel()

	ring := testRing(t)
	var public []string
	for _, route := range Surface() {
		path := strings.Replace(route.Path, "{tool}", "recall_for_purchase", 1)
		body := ""
		if route.ID == RouteTools {
			body = recallBody("shopper_01")
		}
		if route.ID == RouteMCP {
			body = mcpCallBody("recall_for_purchase", `{"shopper_id":"shopper_01"}`)
		}
		recorder, got := serve(ring, route.Method, path, body, "")
		if route.Public {
			public = append(public, route.Path)
			if recorder.Code != 200 {
				t.Fatalf("%s: status = %d, want 200 without a key", route.Path, recorder.Code)
			}
			if !got.reached {
				t.Fatalf("%s: a public route did not reach the layer", route.Path)
			}
			continue
		}
		if recorder.Code != 401 {
			t.Fatalf("%s: status = %d, want 401 without a key", route.Path, recorder.Code)
		}
		if got.reached {
			t.Fatalf("%s: a route without a key reached the layer", route.Path)
		}
		if code := errorCode(t, recorder.Body.Bytes()); code != CodeUnauthenticated {
			t.Fatalf("%s: error code = %q, want %q", route.Path, code, CodeUnauthenticated)
		}
	}
	want := []string{"/healthz", "/webhooks/paypal", "/webhooks/payment"}
	if strings.Join(public, ",") != strings.Join(want, ",") {
		t.Fatalf("public routes = %v, want %v", public, want)
	}
}

// TestForgedIdentityHeadersAreOverwritten proves a client cannot name itself
// to the layer: the guard removes what it sent and sets its own.
func TestForgedIdentityHeadersAreOverwritten(t *testing.T) {
	t.Parallel()

	ring := testRing(t)
	got := &captured{}
	guard := ring.Guard(captureInto(got))
	request := httptest.NewRequest("POST", "/tools/recall_for_purchase", strings.NewReader(recallBody("shopper_01")))
	request.Header.Set("Authorization", "Bearer "+tokenAgentChat)
	request.Header.Set(HeaderCallerKind, "merchant")
	request.Header.Set(HeaderCallerID, "merchant-brandx")
	request.Header.Set(HeaderSeller, "seller-other")
	request.Header.Set(HeaderShopper, "shopper_09")
	recorder := httptest.NewRecorder()
	guard.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got.header.Get(HeaderCallerKind) != string(CallerAgent) {
		t.Fatalf("caller-kind header = %q, want agent", got.header.Get(HeaderCallerKind))
	}
	if got.header.Get(HeaderCallerID) != "agent-chat" {
		t.Fatalf("caller-id header = %q, want agent-chat", got.header.Get(HeaderCallerID))
	}
	if got.header.Get(HeaderSeller) != "seller-demo" {
		t.Fatalf("seller header = %q, want seller-demo", got.header.Get(HeaderSeller))
	}
	if got.header.Get(HeaderShopper) != "shopper_01" {
		t.Fatalf("shopper header = %q, want shopper_01", got.header.Get(HeaderShopper))
	}
}

// TestUnauthenticatedRequestsDoNotReadTheBody proves the guard refuses before
// it buffers anything, so an unauthenticated caller cannot spend the edge's
// memory or change the answer by its body.
func TestUnauthenticatedRequestsDoNotReadTheBody(t *testing.T) {
	t.Parallel()

	ring := testRing(t)
	got := &captured{}
	guard := ring.Guard(captureInto(got))
	reader := &countingReader{reads: 0, body: strings.NewReader(recallBody("shopper_01"))}
	request := httptest.NewRequest("POST", "/tools/recall_for_purchase", reader)
	recorder := httptest.NewRecorder()
	guard.ServeHTTP(recorder, request)

	if recorder.Code != 401 {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if reader.reads != 0 {
		t.Fatalf("the guard read an unauthenticated body %d times", reader.reads)
	}
}

// countingReader counts how many times its body is read.
type countingReader struct {
	reads int
	body  *strings.Reader
}

// Read reads the body once and counts the call.
func (c *countingReader) Read(p []byte) (int, error) {
	c.reads++
	return c.body.Read(p)
}
