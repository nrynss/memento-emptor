package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
)

// TestHTTPTransport proves the service's two surfaces: MCP over JSON-RPC
// and one plain endpoint per tool for the ledger. Both serve the frozen
// contract; tool errors ride inside the MCP result.
func TestHTTPTransport(t *testing.T) {
	svc := seededService(t)
	httpServer := httptest.NewServer(svc.Handler())
	defer httpServer.Close()

	post := func(t *testing.T, path, body string) (int, []byte) {
		t.Helper()
		res, err := http.Post(httpServer.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		defer res.Body.Close()
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return res.StatusCode, raw
	}

	t.Run("health", func(t *testing.T) {
		res, err := http.Get(httpServer.URL + "/healthz")
		if err != nil {
			t.Fatalf("GET /healthz: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("health status %d, want 200", res.StatusCode)
		}
	})

	t.Run("tools_list", func(t *testing.T) {
		status, raw := post(t, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
		if status != http.StatusOK {
			t.Fatalf("tools/list status %d", status)
		}
		var res struct {
			Result struct {
				Tools []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatalf("parse tools/list: %v", err)
		}
		if len(res.Result.Tools) != len(contract.ToolNames) {
			t.Fatalf("tools/list carries %d tools, want %d", len(res.Result.Tools), len(contract.ToolNames))
		}
		names := map[string]bool{}
		for _, tool := range res.Result.Tools {
			names[tool.Name] = true
			if tool.Description == "" {
				t.Errorf("tool %q has no description", tool.Name)
			}
		}
		for _, name := range contract.ToolNames {
			if !names[string(name)] {
				t.Errorf("tools/list misses frozen tool %q", name)
			}
		}
	})

	t.Run("tools_call_recall", func(t *testing.T) {
		req, err := os.ReadFile(filepath.Join(apiDir(t), "recall_for_purchase.request.example.json"))
		if err != nil {
			t.Fatalf("read request fixture: %v", err)
		}
		call, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 2, "method": "tools/call",
			"params": map[string]any{"name": "recall_for_purchase", "arguments": json.RawMessage(req)},
		})
		status, raw := post(t, "/mcp", string(call))
		if status != http.StatusOK {
			t.Fatalf("tools/call status %d", status)
		}
		var res struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatalf("parse tools/call: %v", err)
		}
		if res.Result.IsError {
			t.Fatalf("tools/call recall failed: %s", res.Result.Content[0].Text)
		}
		var actual map[string]any
		if err := json.Unmarshal([]byte(res.Result.Content[0].Text), &actual); err != nil {
			t.Fatalf("parse recall result: %v", err)
		}
		// The base fixture carries the tightened shape: a null pool with
		// an unavailable lookup and every belief's precedence step. The
		// served recall carries the shopper's general statement beside
		// the sizing belief; filter the statement here, as server_test
		// pins it, and compare everything else byte-exact.
		if beliefs, ok := actual["beliefs"].([]any); ok {
			kept := beliefs[:0]
			for _, belief := range beliefs {
				view, ok := belief.(map[string]any)
				if !ok {
					continue
				}
				if view["belief_id"] == "blf_77c310" {
					continue
				}
				kept = append(kept, belief)
			}
			actual["beliefs"] = kept
		}
		wantRaw, _ := os.ReadFile(filepath.Join(apiDir(t), "recall_for_purchase.response.example.json"))
		var want map[string]any
		if err := json.Unmarshal(wantRaw, &want); err != nil {
			t.Fatalf("parse recall fixture: %v", err)
		}
		if !mapsEqual(actual, want) {
			t.Errorf("MCP recall differs from the fixture:\nactual: %s\nwant: %s", pretty(actual), pretty(want))
		}
	})

	t.Run("tools_call_unknown", func(t *testing.T) {
		status, raw := post(t, "/mcp", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
		if status != http.StatusOK {
			t.Fatalf("tools/call status %d", status)
		}
		var res struct {
			Result struct {
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatalf("parse unknown tool call: %v", err)
		}
		if !res.Result.IsError {
			t.Error("unknown tool call is not marked isError")
		}
	})

	t.Run("plain_tool_route", func(t *testing.T) {
		req, err := os.ReadFile(filepath.Join(apiDir(t), "pending_questions.request.example.json"))
		if err != nil {
			t.Fatalf("read request fixture: %v", err)
		}
		status, raw := post(t, "/tools/pending_questions", string(req))
		if status != http.StatusOK {
			t.Fatalf("plain route status %d: %s", status, raw)
		}
		var actual map[string]any
		if err := json.Unmarshal(raw, &actual); err != nil {
			t.Fatalf("parse plain response: %v", err)
		}
		wantRaw, _ := os.ReadFile(filepath.Join(apiDir(t), "pending_questions.response.example.json"))
		var want map[string]any
		if err := json.Unmarshal(wantRaw, &want); err != nil {
			t.Fatalf("parse pending fixture: %v", err)
		}
		if !mapsEqual(actual, want) {
			t.Errorf("plain pending_questions differs:\nactual: %s\nwant: %s", pretty(actual), pretty(want))
		}
	})

	t.Run("calls_log", func(t *testing.T) {
		res, err := http.Get(httpServer.URL + "/calls?trace_id=trace_3f8a01")
		if err != nil {
			t.Fatalf("GET /calls: %v", err)
		}
		defer res.Body.Close()
		var log struct {
			Calls []CallEntry `json:"calls"`
		}
		if err := json.NewDecoder(res.Body).Decode(&log); err != nil {
			t.Fatalf("parse calls: %v", err)
		}
		if len(log.Calls) != 1 || log.Calls[0].Tool != "recall_for_purchase" {
			t.Fatalf("calls for trace_3f8a01 holds %v, want the 1 MCP recall", log.Calls)
		}
	})
}

// mapsEqual compares two decoded JSON values.
func mapsEqual(actual, want map[string]any) bool {
	if len(actual) != len(want) {
		return false
	}
	for key, w := range want {
		a, ok := actual[key]
		if !ok {
			return false
		}
		ar, _ := json.Marshal(a)
		wr, _ := json.Marshal(w)
		if string(ar) != string(wr) {
			return false
		}
	}
	return true
}
