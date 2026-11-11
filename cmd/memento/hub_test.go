package main

// The hub client parses replies the MCP hub sends, and those shapes are the
// contract between the two processes. These tests replay the recorded turns
// under testdata/lambo through the client over a fake hub, so a reply shape
// the client depends on cannot drift without a test naming it. Nothing here
// reaches a hub process or the network.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// recordedTurn is one line of a recording under testdata/lambo: the request
// the driver sent and the reply the hub gave.
type recordedTurn struct {
	Method  string `json:"method"`
	Request *struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	} `json:"request"`
	Response json.RawMessage `json:"response"`
}

// toolTurn is one recorded tools/call: the arguments the driver sent and
// the result envelope the hub answered.
type toolTurn struct {
	args  map[string]any
	reply json.RawMessage
}

// toolTurns reads one recording under testdata/lambo and returns its
// tools/call turns for one tool, in recording order.
func toolTurns(t *testing.T, name, tool string) []toolTurn {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "lambo", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read recording %s: %v", path, err)
	}
	var out []toolTurn
	for lineNo, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var turn recordedTurn
		if err := json.Unmarshal([]byte(line), &turn); err != nil {
			t.Fatalf("%s line %d: %v", name, lineNo+1, err)
		}
		if turn.Request == nil || turn.Request.Method != "tools/call" {
			continue
		}
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(turn.Request.Params, &params); err != nil {
			t.Fatalf("%s line %d: %v", name, lineNo+1, err)
		}
		if params.Name != tool {
			continue
		}
		out = append(out, toolTurn{args: params.Arguments, reply: turn.Response})
	}
	if len(out) == 0 {
		t.Fatalf("%s holds no recorded %s call", name, tool)
	}
	return out
}

// argString reads one string argument from a recorded turn.
func (turn toolTurn) argString(t *testing.T, key string) string {
	t.Helper()
	value, ok := turn.args[key].(string)
	if !ok {
		t.Fatalf("recorded call carries no %q argument: %v", key, turn.args)
	}
	return value
}

// hubRequest is the part of one MCP POST a fake hub routes on.
type hubRequest struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
}

// replayHub is an MCP hub stand-in that answers the open handshake and then
// serves one recorded reply per tool, in order. A tool it holds no reply
// for answers 500, so a client that calls more than the recording covers
// fails loudly instead of reusing a reply.
type replayHub struct {
	server *httptest.Server

	mu      sync.Mutex
	replies map[string][]json.RawMessage
}

// newReplayHub starts a fake hub holding one queue of replies per tool name.
func newReplayHub(t *testing.T, replies map[string][]json.RawMessage) *replayHub {
	t.Helper()
	hub := &replayHub{replies: replies}
	hub.server = httptest.NewServer(http.HandlerFunc(hub.serve))
	t.Cleanup(hub.server.Close)
	return hub
}

// graph returns a client pointed at this hub, as the layer builds it for
// one shopper.
func (h *replayHub) graph() *hubGraph {
	return newHubGraph(h.server.URL, "test-token", "shopper-test")
}

// serve answers one MCP POST.
func (h *replayHub) serve(w http.ResponseWriter, r *http.Request) {
	var request hubRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch request.Method {
	case "initialize":
		writeInitialize(w, request.ID)
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/call":
		h.mu.Lock()
		queue := h.replies[request.Params.Name]
		if len(queue) == 0 {
			h.mu.Unlock()
			http.Error(w, "no recorded reply for "+request.Params.Name, http.StatusInternalServerError)
			return
		}
		reply := queue[0]
		h.replies[request.Params.Name] = queue[1:]
		h.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(reply)
	default:
		http.Error(w, "unknown method", http.StatusBadRequest)
	}
}

// writeInitialize answers the MCP open with a session header, which is how
// the hub attaches a client to the addressed session.
func writeInitialize(w http.ResponseWriter, id json.RawMessage) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Mcp-Session-Id", "test-session")
	_, _ = fmt.Fprintf(w,
		`{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"fake-hub","version":"test"}}}`,
		id)
}

// TestInspectPinsRecordedReplies replays every recorded lambo_inspect turn
// through the client and pins the report it builds from it: the admission
// status, the checks the client reads, the blast radius, the warnings, and
// the no-concept-matching mapping.
func TestInspectPinsRecordedReplies(t *testing.T) {
	cases := []struct {
		name  string
		file  string
		index int
		want  fakes.BeliefReport
	}{
		{
			name:  "seeded belief is venerable with its admission checks",
			file:  "01-seed.jsonl",
			index: 0,
			want: fakes.BeliefReport{
				State:                domain.GraphVenerable,
				Dependents:           3,
				DistinctInteractions: 3,
				Coverage:             2.0 / 3.0,
			},
		},
		{
			name:  "correction warns beside the kept belief",
			file:  "02-correction-dispute.jsonl",
			index: 0,
			want: fakes.BeliefReport{
				State:                domain.GraphVenerable,
				Dependents:           3,
				DistinctInteractions: 3,
				Coverage:             2.0 / 3.0,
				Contradicted:         true,
				Warning:              `Contradicted by ⚠ Contradicted at 2026-11-09T16:46:55Z by memento-t03: "Shopper correction: lost weight, take size M for Brand X" (id 73a96fc2)`,
			},
		},
		{
			name:  "a later inspect reads the recalculated coverage",
			file:  "02-correction-dispute.jsonl",
			index: 1,
			want: fakes.BeliefReport{
				State:                domain.GraphVenerable,
				Dependents:           3,
				DistinctInteractions: 3,
				Coverage:             0.599947147711573,
				Contradicted:         true,
				Warning:              `Contradicted by ⚠ Contradicted at 2026-11-09T16:46:55Z by memento-t03: "Shopper correction: lost weight, take size M for Brand X" (id 73a96fc2)`,
			},
		},
		{
			name:  "unpromoted belief reads None with its one dependent",
			file:  "02-correction-dispute.jsonl",
			index: 2,
			want: fakes.BeliefReport{
				State:      domain.GraphNone,
				Dependents: 1,
			},
		},
		{
			name:  "assertion reads None with no dependents",
			file:  "02-correction-dispute.jsonl",
			index: 3,
			want: fakes.BeliefReport{
				State: domain.GraphNone,
			},
		},
		{
			name:  "a focus the hub never saw maps to None without an error",
			file:  "03-second-session.jsonl",
			index: 0,
			want: fakes.BeliefReport{
				State: domain.GraphNone,
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			turns := toolTurns(t, testCase.file, "lambo_inspect")
			if testCase.index >= len(turns) {
				t.Fatalf("%s holds %d inspect turns, want index %d", testCase.file, len(turns), testCase.index)
			}
			turn := turns[testCase.index]
			focus := turn.argString(t, "focus")
			hub := newReplayHub(t, map[string][]json.RawMessage{"lambo_inspect": {turn.reply}})
			report, err := hub.graph().Inspect(context.Background(), focus)
			if err != nil {
				t.Fatalf("inspect %q: %v", focus, err)
			}
			if report.Content != focus {
				t.Errorf("content = %q, want the inspected focus %q", report.Content, focus)
			}
			if math.Abs(report.Coverage-testCase.want.Coverage) > 1e-12 {
				t.Errorf("coverage = %v, want %v", report.Coverage, testCase.want.Coverage)
			}
			got, want := report, testCase.want
			got.Coverage, want.Coverage = 0, 0
			got.Content, want.Content = "", ""
			if got != want {
				t.Errorf("report = %+v, want %+v", report, testCase.want)
			}
		})
	}
}

// TestInspectNoConceptErrorMapsToNone pins the error-envelope half of the
// no-concept-matching mapping: the hub may answer a JSON-RPC error carrying
// the same message the recorded text does, and the client reads that as a
// belief that never promoted, not as a failure. The recorded text branch
// cannot distinguish this substring from its fallback, so this pins the
// branch on its own.
func TestInspectNoConceptErrorMapsToNone(t *testing.T) {
	message := "lambo_inspect: no concept matching 'Brand X oxford runs small for this shopper: take size L' in session 't03-shopper-2'"
	reply := json.RawMessage(fmt.Sprintf(
		`{"jsonrpc":"2.0","id":4,"error":{"code":-32000,"message":%q}}`, message))
	// A JSON-RPC error is a request failure, so the client re-opens and
	// retries once; the fake hub answers the same envelope both times.
	hub := newReplayHub(t, map[string][]json.RawMessage{"lambo_inspect": {reply, reply}})
	focus := "Brand X oxford runs small for this shopper: take size L"
	report, err := hub.graph().Inspect(context.Background(), focus)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	want := fakes.BeliefReport{Content: focus, State: domain.GraphNone}
	if report != want {
		t.Fatalf("report = %+v, want %+v", report, want)
	}
}

// TestInspectStructuredStatusWins pins the one inspect reading the
// recordings cannot separate: a real reply renders the state in
// structuredContent.admission.status and in the text mark from the same
// value, so either alone covers the recorded cases. These replies name
// different states in the two places on purpose, so a client that stopped
// reading the structured status (or no longer maps one of its values) fails
// here instead of falling back to the text unnoticed.
func TestInspectStructuredStatusWins(t *testing.T) {
	cases := []struct {
		name   string
		status string
		mark   string
		want   domain.GraphState
	}{
		{name: "venerable status over a candidate text mark", status: "Venerable", mark: "candidate", want: domain.GraphVenerable},
		{name: "candidate status over a venerable text mark", status: "Candidate", mark: "venerable", want: domain.GraphCandidate},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			reply := json.RawMessage(fmt.Sprintf(
				`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"focus: a belief [Logic, %s]"}],"structuredContent":{"admission":{"status":%q},"focus":{"blast_radius":0},"warnings":[]},"isError":false}}`,
				testCase.mark, testCase.status))
			hub := newReplayHub(t, map[string][]json.RawMessage{"lambo_inspect": {reply}})
			report, err := hub.graph().Inspect(context.Background(), "a belief")
			if err != nil {
				t.Fatalf("inspect: %v", err)
			}
			if report.State != testCase.want {
				t.Fatalf("state = %q, want %q", report.State, testCase.want)
			}
		})
	}
}

// TestWriterRepliesCarryReceipts replays every recorded write and its
// settling stats poll through the client and pins the receipt it reads and
// the applied state it waits out.
func TestWriterRepliesCarryReceipts(t *testing.T) {
	cases := []struct {
		name       string
		file       string
		tool       string
		index      int
		statsIndex int
		receipt    string
	}{
		{
			name:       "derive receipt",
			file:       "01-seed.jsonl",
			tool:       "lambo_derive",
			index:      0,
			statsIndex: 0,
			receipt:    "lwr1.3eae3fd062ef3014.1a1c1337e39.1",
		},
		{
			name:       "record action receipt",
			file:       "01-seed.jsonl",
			tool:       "lambo_record_action",
			index:      0,
			statsIndex: 2,
			receipt:    "lwr1.3eae3fd062ef3014.1a1c1337f02.3",
		},
		{
			name:       "correction receipt",
			file:       "02-correction-dispute.jsonl",
			tool:       "lambo_record_action",
			index:      0,
			statsIndex: 0,
			receipt:    "lwr1.521be39055389568.1a1c134a5f8.1",
		},
		{
			name:       "derive receipt for the rewritten belief",
			file:       "02-correction-dispute.jsonl",
			tool:       "lambo_derive",
			index:      0,
			statsIndex: 1,
			receipt:    "lwr1.521be39055389568.1a1c134a6c0.2",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			writes := toolTurns(t, testCase.file, testCase.tool)
			settles := toolTurns(t, testCase.file, "lambo_stats")
			if testCase.index >= len(writes) || testCase.statsIndex >= len(settles) {
				t.Fatalf("%s holds %d %s turns and %d stats turns", testCase.file, len(writes), testCase.tool, len(settles))
			}
			turn := writes[testCase.index]
			hub := newReplayHub(t, map[string][]json.RawMessage{
				testCase.tool: {turn.reply},
				"lambo_stats": {settles[testCase.statsIndex].reply},
			})
			graph := hub.graph()
			ctx := context.Background()
			receipt, err := graph.callTool(ctx, testCase.tool, turn.args)
			if err != nil {
				t.Fatalf("%s: %v", testCase.tool, err)
			}
			if receipt != testCase.receipt {
				t.Errorf("receipt = %q, want %q", receipt, testCase.receipt)
			}
			if err := graph.waitApplied(ctx, receipt, testCase.tool); err != nil {
				t.Errorf("wait applied %s: %v", testCase.receipt, err)
			}
		})
	}
}

// TestStatsReplyIsNotAWriteReceipt pins the one shape difference inside the
// writer contract: lambo_stats returns its settled receipt as an object,
// while a write returns it as a string. A client that read the stats shape
// as a write receipt would wait on a garbage receipt.
func TestStatsReplyIsNotAWriteReceipt(t *testing.T) {
	stats := toolTurns(t, "01-seed.jsonl", "lambo_stats")[0]
	hub := newReplayHub(t, map[string][]json.RawMessage{"lambo_stats": {stats.reply}})
	receipt, err := hub.graph().callTool(context.Background(), "lambo_stats", stats.args)
	if err == nil {
		t.Fatalf("stats reply read as a write receipt %q", receipt)
	}
	if !strings.Contains(err.Error(), "issued no receipt") {
		t.Fatalf("error = %v, want the no-receipt refusal", err)
	}
}

// TestRecallPinsRecordedReplies replays recorded recalls and pins the hit
// labels the client splits out of the text content.
func TestRecallPinsRecordedReplies(t *testing.T) {
	t.Run("ranked hits", func(t *testing.T) {
		turn := toolTurns(t, "01-seed.jsonl", "lambo_recall")[0]
		hub := newReplayHub(t, map[string][]json.RawMessage{"lambo_recall": {turn.reply}})
		labels, err := hub.graph().Recall(context.Background(), turn.argString(t, "query"))
		if err != nil {
			t.Fatalf("recall: %v", err)
		}
		want := []string{
			"Brand X oxford runs small for this shopper: take size L [Logic] (score 1.71, id c1360ff5)",
			"Shopper says they are usually size M [Observation] (score 1.14, id 8d82231d)",
			"Fit return 1: Brand X oxford shirt size M refunded; reason too tight in the shoulders [Resource] (score 0.77, id 1fe8188c)",
			"Fit return 3: Brand X oxford shirt size M refunded; reason sleeves too short in M [Resource] (score 0.75, id 4813e621)",
			"Fit return 2: Brand X oxford shirt size M refunded; reason too tight across the chest [Resource] (score 0.75, id c522ad2f)",
		}
		if !slices.Equal(labels, want) {
			t.Fatalf("labels = %#v, want %#v", labels, want)
		}
	})

	t.Run("a session with no hits", func(t *testing.T) {
		turn := toolTurns(t, "03-second-session.jsonl", "lambo_recall")[0]
		hub := newReplayHub(t, map[string][]json.RawMessage{"lambo_recall": {turn.reply}})
		labels, err := hub.graph().Recall(context.Background(), turn.argString(t, "query"))
		if err != nil {
			t.Fatalf("recall: %v", err)
		}
		if len(labels) != 0 {
			t.Fatalf("labels = %#v, want none", labels)
		}
	})
}
