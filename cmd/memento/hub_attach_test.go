package main

// The hub bounds how many sessions stay attached: a request that finds
// every max_attached place taken is refused with HTTP 503 and a
// Retry-After, and only retrying the attach can relieve it. These tests
// drive that path over a fake hub and pin the delay the client honours,
// the bound on its attempts, and the typed error a hub that stays full
// surfaces.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// attachAnswer is one canned reply for the hub's open handshake: the status
// it answers with and the Retry-After it carries.
type attachAnswer struct {
	status     int
	retryAfter string
}

// boundHub is a hub stand-in that answers the open handshake from a canned
// list, repeating the last answer when the list runs out. It counts the
// requests that reached it, so a test can prove the client's retries are
// bounded. Tools/call answers the configured status: 200 carries a receipt,
// anything else carries no body the client could read.
type boundHub struct {
	server *httptest.Server

	mu         sync.Mutex
	answers    []attachAnswer
	callStatus int
	callRetry  string
	initCount  int
	callCount  int
}

// newBoundHub starts a fake hub with canned handshake answers and one
// status for every tools/call.
func newBoundHub(t *testing.T, answers []attachAnswer, callStatus int, callRetryAfter string) *boundHub {
	t.Helper()
	hub := &boundHub{answers: answers, callStatus: callStatus, callRetry: callRetryAfter}
	hub.server = httptest.NewServer(http.HandlerFunc(hub.serve))
	t.Cleanup(hub.server.Close)
	return hub
}

// graph returns a client pointed at this hub, as the layer builds it for
// one shopper.
func (h *boundHub) graph() *hubGraph {
	return newHubGraph(h.server.URL, "test-token", "shopper-test")
}

// counts reports how many opens and tool calls the hub answered.
func (h *boundHub) counts() (opens, calls int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.initCount, h.callCount
}

// serve answers one MCP POST.
func (h *boundHub) serve(w http.ResponseWriter, r *http.Request) {
	var request hubRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch request.Method {
	case "initialize":
		h.mu.Lock()
		answer := attachAnswer{status: http.StatusOK}
		if len(h.answers) > 0 {
			index := h.initCount
			if index >= len(h.answers) {
				index = len(h.answers) - 1
			}
			answer = h.answers[index]
		}
		h.initCount++
		h.mu.Unlock()
		if answer.retryAfter != "" {
			w.Header().Set("Retry-After", answer.retryAfter)
		}
		if answer.status != http.StatusOK {
			http.Error(w, "attach bound", answer.status)
			return
		}
		writeInitialize(w, request.ID)
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/call":
		h.mu.Lock()
		h.callCount++
		status, retry := h.callStatus, h.callRetry
		h.mu.Unlock()
		if retry != "" {
			w.Header().Set("Retry-After", retry)
		}
		if status != 0 && status != http.StatusOK {
			http.Error(w, "call refused", status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w,
			`{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"accepted 1 concept(s) for background write"}],"structuredContent":{"receipt":"%s","receipt_state":"pending","warnings":[]},"isError":false}}`,
			request.ID, testReceipt)
	default:
		http.Error(w, "unknown method", http.StatusBadRequest)
	}
}

// testReceipt is the receipt the fake hub hands back on a successful write.
const testReceipt = "lwr1.test.1"

// TestCallHonoursAttachBound table-tests the client against a hub that
// refuses the attach: the refusal is retried after the delay the hub
// stated, the attempts are bounded, and a hub that stays full surfaces the
// typed error instead of looping.
func TestCallHonoursAttachBound(t *testing.T) {
	cases := []struct {
		name string
		// answers is the handshake's canned replies, last one repeating.
		answers []attachAnswer
		// callStatus is the tools/call answer; 0 reads as 200.
		callStatus int
		// callRetry is the Retry-After carried on a refused tool call.
		callRetry string
		// wantBound asks for the typed error; wantError asks for any other.
		wantBound bool
		wantError bool
		// wantOpens and wantCalls pin the request count, so a retry storm
		// fails the test rather than slowing it.
		wantOpens int
		wantCalls int
		// wantWaits pins each delay the client spent, in order.
		wantWaits []time.Duration
	}{
		{
			name:      "a healthy hub opens once and calls once",
			answers:   []attachAnswer{{status: http.StatusOK}},
			wantOpens: 1,
			wantCalls: 1,
		},
		{
			name: "refused once then attached",
			answers: []attachAnswer{
				{status: http.StatusServiceUnavailable, retryAfter: "0"},
				{status: http.StatusOK},
			},
			wantOpens: 2,
			wantCalls: 1,
			wantWaits: []time.Duration{0},
		},
		{
			name: "refused with a stated delay",
			answers: []attachAnswer{
				{status: http.StatusServiceUnavailable, retryAfter: "2"},
				{status: http.StatusOK},
			},
			wantOpens: 2,
			wantCalls: 1,
			wantWaits: []time.Duration{2 * time.Second},
		},
		{
			name: "refusal without a header waits the default",
			answers: []attachAnswer{
				{status: http.StatusServiceUnavailable},
				{status: http.StatusOK},
			},
			wantOpens: 2,
			wantCalls: 1,
			wantWaits: []time.Duration{attachRetryDefault},
		},
		{
			name: "a hub that stays full surfaces the bound and posts no call",
			answers: []attachAnswer{
				{status: http.StatusServiceUnavailable, retryAfter: "0"},
			},
			wantBound: true,
			wantOpens: attachAttempts,
			wantCalls: 0,
			wantWaits: []time.Duration{0, 0},
		},
		{
			name:       "a refused call is retried after the stated delay",
			answers:    []attachAnswer{{status: http.StatusOK}},
			callStatus: http.StatusServiceUnavailable,
			callRetry:  "1",
			wantBound:  true,
			wantOpens:  attachAttempts,
			wantCalls:  attachAttempts,
			wantWaits:  []time.Duration{time.Second, time.Second},
		},
		{
			name: "a server error is not the bound",
			answers: []attachAnswer{
				{status: http.StatusInternalServerError},
			},
			wantError: true,
			wantOpens: 1,
			wantCalls: 0,
		},
		{
			name:       "a call failure keeps the one re-attach retry",
			answers:    []attachAnswer{{status: http.StatusOK}},
			callStatus: http.StatusInternalServerError,
			wantError:  true,
			wantOpens:  2,
			wantCalls:  2,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			hub := newBoundHub(t, testCase.answers, testCase.callStatus, testCase.callRetry)
			graph := hub.graph()
			var waits []time.Duration
			graph.wait = func(ctx context.Context, delay time.Duration) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				waits = append(waits, delay)
				return nil
			}
			receipt, err := graph.callTool(context.Background(), "lambo_derive", map[string]any{
				"agent_id": hubAgent,
				"concepts": []map[string]any{{"content": "a belief", "concept_type": "logic"}},
			})
			switch {
			case testCase.wantBound:
				if err == nil || !errors.Is(err, errAttachBound) {
					t.Fatalf("error = %v, want one matching errAttachBound", err)
				}
				var bound *attachBoundError
				if !errors.As(err, &bound) {
					t.Fatalf("error = %v, want the typed attach refusal", err)
				}
				if len(testCase.wantWaits) > 0 {
					last := testCase.wantWaits[len(testCase.wantWaits)-1]
					if bound.retryAfter != last {
						t.Errorf("refusal carries retry after %s, want the stated %s", bound.retryAfter, last)
					}
				}
			case testCase.wantError:
				if err == nil {
					t.Fatal("error = nil, want a failure")
				}
				if errors.Is(err, errAttachBound) {
					t.Fatalf("error = %v, want not the attach bound", err)
				}
			default:
				if err != nil {
					t.Fatalf("call: %v", err)
				}
				if receipt != testReceipt {
					t.Errorf("receipt = %q, want %q", receipt, testReceipt)
				}
			}
			opens, calls := hub.counts()
			if opens != testCase.wantOpens || calls != testCase.wantCalls {
				t.Errorf("hub saw %d opens and %d calls, want %d and %d", opens, calls, testCase.wantOpens, testCase.wantCalls)
			}
			if !slices.Equal(waits, testCase.wantWaits) {
				t.Errorf("waits = %v, want %v", waits, testCase.wantWaits)
			}
		})
	}
}

// TestCallStopsWaitingWhenTheContextEnds pins that a stated delay is
// bounded by the caller: a request that ends while the client waits for
// the attach returns the context's error, not a hub error.
func TestCallStopsWaitingWhenTheContextEnds(t *testing.T) {
	hub := newBoundHub(t, []attachAnswer{{status: http.StatusServiceUnavailable, retryAfter: "30"}}, 0, "")
	graph := hub.graph()
	ctx, cancel := context.WithCancel(context.Background())
	graph.wait = func(ctx context.Context, delay time.Duration) error {
		cancel()
		return waitContext(ctx, delay)
	}
	_, err := graph.callTool(ctx, "lambo_derive", map[string]any{"agent_id": hubAgent})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want the context's error", err)
	}
	if opens, calls := hub.counts(); opens != 1 || calls != 0 {
		t.Fatalf("hub saw %d opens and %d calls, want 1 and 0", opens, calls)
	}
}

// TestAttachRetryReadsSeconds pins how a refusal's Retry-After becomes a
// delay: whole seconds when the header carries a count, the default
// otherwise, and never a negative wait.
func TestAttachRetryReadsSeconds(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{name: "absent", header: "", want: attachRetryDefault},
		{name: "zero", header: "0", want: 0},
		{name: "seconds", header: "90", want: 90 * time.Second},
		{name: "negative", header: "-3", want: attachRetryDefault},
		{name: "a date", header: "Wed, 21 Oct 2026 07:28:00 GMT", want: attachRetryDefault},
		{name: "a fraction", header: "1.5", want: attachRetryDefault},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			header := http.Header{}
			if testCase.header != "" {
				header.Set("Retry-After", testCase.header)
			}
			if got := attachRetry(header); got != testCase.want {
				t.Fatalf("delay = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWaitContextSpendsTheDelay pins the wait the client actually runs: it
// spends the stated delay, spends nothing for a zero delay, and returns the
// context's error when the context ends first.
func TestWaitContextSpendsTheDelay(t *testing.T) {
	t.Run("stated delay", func(t *testing.T) {
		const delay = 20 * time.Millisecond
		start := time.Now()
		if err := waitContext(context.Background(), delay); err != nil {
			t.Fatalf("wait: %v", err)
		}
		if elapsed := time.Since(start); elapsed < delay {
			t.Fatalf("waited %s, want at least %s", elapsed, delay)
		}
	})

	t.Run("no delay", func(t *testing.T) {
		start := time.Now()
		if err := waitContext(context.Background(), 0); err != nil {
			t.Fatalf("wait: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
			t.Fatalf("waited %s for a zero delay", elapsed)
		}
	})

	t.Run("context ends first", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := waitContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want the context's error", err)
		}
	})
}

// TestAttachBoundErrorMatchesTheSentinel pins the typed refusal's contract:
// callers detect it with errors.Is, read the stated delay with errors.As,
// and the message names the bound.
func TestAttachBoundErrorMatchesTheSentinel(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &attachBoundError{
		tool:       "initialize",
		status:     http.StatusServiceUnavailable,
		retryAfter: 3 * time.Second,
		excerpt:    "attach bound",
	})
	if !errors.Is(err, errAttachBound) {
		t.Fatal("errors.Is(err, errAttachBound) = false")
	}
	var bound *attachBoundError
	if !errors.As(err, &bound) {
		t.Fatal("errors.As found no attachBoundError")
	}
	if bound.retryAfter != 3*time.Second {
		t.Errorf("retryAfter = %s, want 3s", bound.retryAfter)
	}
	for _, want := range []string{"attach bound", "HTTP 503", "retry after 3s"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message = %q, want it to name %q", err.Error(), want)
		}
	}
}
